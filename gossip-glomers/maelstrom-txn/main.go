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
	mu       sync.Mutex
	kv       map[int]int
	versions map[int]int
}

func (s *NodeState) handleTxn(n *maelstrom.Node, txn [][]any) ([][]any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reads := make(map[int]int)
	writes := make(map[int]int)

	for _, op := range txn {
		opType := op[0].(string)
		key := int(op[1].(float64))
		switch opType {
		case "r":
			value := s.kv[key]
			version := s.versions[key]
			op[2] = value
			reads[key] = version
		case "w":
			value := int(op[2].(float64))
			writes[key] = value
		}
	}

	for key, readVersion := range reads {
		if readVersion < s.versions[key] {
			return nil, false
		}
	}
	for k, v := range writes {
		s.kv[k] = v
		s.versions[k]++
	}
	return txn, true
}

func (s *NodeState) broadcastTxn(n *maelstrom.Node, txn [][]any) error {
	for _, node := range n.NodeIDs() {
		if node == n.ID() {
			continue
		}
		go func(dest string) {
			for {
				err := n.Send(dest, map[string]any{
					"type": "txn",
					"txn":  txn,
				})
				if err == nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(node)
	}
	return nil
}

func main() {
	n := maelstrom.NewNode()
	state := NodeState{
		kv:       make(map[int]int),
		versions: make(map[int]int),
	}

	n.Handle("txn", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}
		txn := body["txn"].([]any)
		var txnOps [][]any
		for _, op := range txn {
			txnOps = append(txnOps, op.([]any))
		}
		updatedTxn, ok := state.handleTxn(n, txnOps)
		if !ok {
			return n.Reply(msg, map[string]any{
				"type": "error",
				"code": maelstrom.TxnConflict,
				"text": "transaction conflict",
			})
		}
		if msg.Src == "c" {
			go state.broadcastTxn(n, updatedTxn)
		}

		body["type"] = "txn_ok"
		return n.Reply(msg, map[string]any{
			"type": "txn_ok",
			"txn":  updatedTxn,
		})
	})

	n.Handle("txn_ok", func(msg maelstrom.Message) error {
		return nil
	})

	if err := n.Run(); err != nil {
		log.Printf("ERROR: %s", err)
		os.Exit(1)
	}
}
