package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type NodeState struct {
	kv maelstrom.KV
}

func (s *NodeState) getShard(n *maelstrom.Node, key string) string {
	h := fnv.New32a()
	h.Write([]byte(key))
	nodes := n.NodeIDs()
	index := h.Sum32() % uint32(len(nodes))
	return nodes[index]
}

func main() {
	n := maelstrom.NewNode()
	state := NodeState{
		kv: *maelstrom.NewLinKV(n),
	}

	n.Handle("send", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		k := body["key"].(string)
		shard := state.getShard(n, k)
		if shard != string(n.ID()) {
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				response, err := n.SyncRPC(ctx, shard, map[string]any{
					"type": "send",
					"key":  k,
					"msg":  body["msg"],
				})
				cancel()
				if err != nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				var responseBody map[string]any
				if err := json.Unmarshal(response.Body, &responseBody); err != nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return n.Reply(msg, responseBody)
			}
		}
		key := fmt.Sprintf("log/%s", k)
		message := int(body["msg"].(float64))

		var offset int
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			var log [][]any
			err := state.kv.ReadInto(ctx, key, &log)
			cancel()

			if err != nil {
				var rpcErr *maelstrom.RPCError
				if errors.As(err, &rpcErr) && rpcErr.Code == maelstrom.KeyDoesNotExist {
					log = make([][]any, 0)
				} else {
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}

			offset = len(log)
			ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
			err = state.kv.CompareAndSwap(ctx, key, log, append(log, []any{offset, message}), true)
			cancel()
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		return n.Reply(msg, map[string]any{
			"type":   "send_ok",
			"offset": offset,
		})
	})

	n.Handle("poll", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		polled := make(map[string][][]any)
		offsets := body["offsets"].(map[string]any)
		ch := make(chan [2]any, len(offsets))

		for k, v := range offsets {
			go func(key string, clientOffset int) {
				serverKey := fmt.Sprintf("log/%s", key)
				for {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					var log [][]any
					err := state.kv.ReadInto(ctx, serverKey, &log)
					cancel()
					if err != nil {
						var rpcErr *maelstrom.RPCError
						if errors.As(err, &rpcErr) && rpcErr.Code == maelstrom.KeyDoesNotExist {
							log = make([][]any, 0)
						} else {
							time.Sleep(100 * time.Millisecond)
							continue
						}
					}
					var messages [][]any
					if clientOffset < len(log) {
						messages = log[clientOffset:]
					} else {
						messages = [][]any{}
					}
					ch <- [2]any{key, messages}
					return
				}
			}(k, int(v.(float64)))
		}
		for i := 0; i < len(offsets); i++ {
			entry := <-ch
			polled[entry[0].(string)] = entry[1].([][]any)
		}
		return n.Reply(msg, map[string]any{
			"type": "poll_ok",
			"msgs": polled,
		})
	})

	n.Handle("commit_offsets", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		offsets := body["offsets"].(map[string]any)
		ch := make(chan bool, len(offsets))

		for k, v := range offsets {
			go func(key string, clientOffset int) {
				serverKey := fmt.Sprintf("offset/%s", key)
				for {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					serverOffset, err := state.kv.ReadInt(ctx, serverKey)
					cancel()
					var rpcErr *maelstrom.RPCError
					if err != nil && errors.As(err, &rpcErr) && rpcErr.Code != maelstrom.KeyDoesNotExist {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					if serverOffset >= clientOffset {
						ch <- true
						return
					}
					ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
					err = state.kv.CompareAndSwap(ctx, serverKey, serverOffset, clientOffset, true)
					cancel()
					if err == nil {
						ch <- true
						return
					}
					time.Sleep(100 * time.Millisecond)
				}
			}(k, int(v.(float64)))
		}
		for i := 0; i < len(offsets); i++ {
			<-ch
		}
		return n.Reply(msg, map[string]any{
			"type": "commit_offsets_ok",
		})
	})

	n.Handle("list_committed_offsets", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		keys := body["keys"].([]any)
		ch := make(chan []any, len(keys))

		for _, k := range keys {
			go func(key string) {
				serverKey := fmt.Sprintf("offset/%s", key)
				for {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					offset, err := state.kv.ReadInt(ctx, serverKey)
					cancel()
					var rpcErr *maelstrom.RPCError
					if err != nil && errors.As(err, &rpcErr) && rpcErr.Code != maelstrom.KeyDoesNotExist {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					ch <- []any{key, offset}
					return
				}
			}(k.(string))
		}

		offsets := make(map[string]int)
		for i := 0; i < len(keys); i++ {
			entry := <-ch
			offsets[entry[0].(string)] = entry[1].(int)
		}
		return n.Reply(msg, map[string]any{
			"type":    "list_committed_offsets_ok",
			"offsets": offsets,
		})
	})

	if err := n.Run(); err != nil {
		log.Printf("ERROR: %s", err)
		os.Exit(1)
	}
}
