package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type NodeState struct {
	kv maelstrom.KV
}

func main() {
	n := maelstrom.NewNode()
	state := NodeState{
		kv: *maelstrom.NewSeqKV(n),
	}

	n.Handle("add", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		delta := int(body["delta"].(float64))
		state.add(n, delta)

		return n.Reply(msg, map[string]any{
			"type": "add_ok",
		})
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		counter := state.read(n)

		return n.Reply(msg, map[string]any{
			"type":  "read_ok",
			"value": counter,
		})
	})

	if err := n.Run(); err != nil {
		log.Printf("ERROR: %s", err)
		os.Exit(1)
	}
}

func (s *NodeState) add(n *maelstrom.Node, delta int) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		val, err := s.kv.ReadInt(ctx, string(n.ID()))
		cancel()

		var rpcErr *maelstrom.RPCError
		if errors.As(err, &rpcErr) && rpcErr.Code != maelstrom.KeyDoesNotExist {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
		err = s.kv.CompareAndSwap(ctx, string(n.ID()), val, val+delta, true)
		cancel()

		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return
	}
}

func (s *NodeState) read(n *maelstrom.Node) int {
	nodeIds := n.NodeIDs()
	ch := make(chan int, len(nodeIds))
	for _, neighbor := range nodeIds {
		go func(key string) {
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				val, err := s.kv.ReadInt(ctx, key)
				cancel()
				if err == nil {
					ch <- val
					return
				}
				var rpcErr *maelstrom.RPCError
				if errors.As(err, &rpcErr) && rpcErr.Code == maelstrom.KeyDoesNotExist {
					ch <- 0
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(string(neighbor))
	}

	counter := 0
	for i := 0; i < len(nodeIds); i++ {
		val := <-ch
		counter += val
	}
	return counter
}
