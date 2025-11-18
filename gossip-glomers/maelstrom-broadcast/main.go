package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type NodeState struct {
	neighbors []string
	seen      map[int]bool
	mu        sync.Mutex
}

func main() {
	n := maelstrom.NewNode()
	state := NodeState{
		seen: make(map[int]bool),
	}

	go state.gossip(n)

	n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		newMessage := false
		state.mu.Lock()
		if msg, ok := body["message"].(float64); ok {
			if !state.seen[int(msg)] {
				state.seen[int(msg)] = true
				newMessage = true
			}
		}
		if msgs, ok := body["messages"].([]any); ok {
			for _, m := range msgs {
				if msgNum, ok := m.(float64); ok {
					if !state.seen[int(msgNum)] {
						state.seen[int(msgNum)] = true
						newMessage = true
					}
				}
			}
		}
		state.mu.Unlock()

		if newMessage {
			state.broadcastAll(n, msg.Src)
		}

		if _, ok := body["msg_id"]; ok {
			return n.Reply(msg, map[string]any{
				"type": "broadcast_ok",
			})
		}
		return nil
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		return n.Reply(msg, map[string]any{
			"type":     "read_ok",
			"messages": state.getMessages(),
		})
	})

	n.Handle("topology", func(msg maelstrom.Message) error {
		neighbors := buildTopology(n.ID(), n.NodeIDs())

		state.mu.Lock()
		state.neighbors = neighbors
		state.mu.Unlock()

		return n.Reply(msg, map[string]any{
			"type": "topology_ok",
		})
	})

	if err := n.Run(); err != nil {
		log.Printf("ERROR: %s", err)
		os.Exit(1)
	}
}

func (s *NodeState) getMessages() []int {
	s.mu.Lock()
	messages := make([]int, 0, len(s.seen))
	for msg := range s.seen {
		messages = append(messages, msg)
	}
	s.mu.Unlock()
	return messages
}

func (s *NodeState) gossip(n *maelstrom.Node) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		s.broadcastAll(n, n.ID())
	}
}

func (s *NodeState) broadcastAll(n *maelstrom.Node, sourceNode string) {
	messages := s.getMessages()
	if len(messages) == 0 {
		return
	}
	s.mu.Lock()
	neighbors := make([]string, len(s.neighbors))
	copy(neighbors, s.neighbors)
	s.mu.Unlock()
	for _, neighbor := range neighbors {
		if neighbor == sourceNode {
			continue
		}
		go func(dest string) {
			n.Send(dest, map[string]any{
				"type":     "broadcast",
				"messages": messages,
			})
		}(neighbor)
	}
}

func buildTopology(id string, nodeIds []string) []string {
	const branchingFactor = 5

	nodeIndex := -1
	for i, nodeId := range nodeIds {
		if nodeId == id {
			nodeIndex = i
			break
		}
	}

	neighbors := make([]string, 0)

	if nodeIndex > 0 {
		parentIndex := (nodeIndex - 1) / branchingFactor
		neighbors = append(neighbors, nodeIds[parentIndex])
	}
	for i := 0; i < branchingFactor; i++ {
		childIndex := nodeIndex*branchingFactor + i + 1
		if childIndex < len(nodeIds) {
			neighbors = append(neighbors, nodeIds[childIndex])
		}
	}

	return neighbors
}
