package shardgrp

import (
	"math/rand"
	"sync"
	"time"

	"6.5840/kvsrv1/rpc"
	"6.5840/shardkv1/shardcfg"
	tester "6.5840/tester1"
)

type Clerk struct {
	clnt    *tester.Clnt
	servers []string
	// You will have to modify this struct.
	mu        sync.Mutex
	leader    int
	clientId  int
	requestId int
}

func MakeClerk(clnt *tester.Clnt, servers []string) *Clerk {
	ck := &Clerk{clnt: clnt, servers: servers, leader: rand.Intn(len(servers))}
	ck.clientId = rand.Int()
	ck.requestId = 0
	return ck
}

func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	// Your code here
	ck.mu.Lock()
	requestId := ck.requestId
	ck.requestId++
	ck.mu.Unlock()
	args := &rpc.GetArgs{Key: key, ClientId: ck.clientId, RequestId: requestId}
	for {
		ck.mu.Lock()
		startLeader := ck.leader
		ck.mu.Unlock()

		for i := 0; i < len(ck.servers); i++ {
			reply := &rpc.GetReply{}
			index := (startLeader + i) % len(ck.servers)
			server := ck.servers[index]
			ok := ck.clnt.Call(server, "KVServer.Get", args, reply)
			if ok && reply.Err == rpc.OK {
				ck.mu.Lock()
				ck.leader = index
				ck.mu.Unlock()
				return reply.Value, reply.Version, reply.Err
			}
			if ok && reply.Err == rpc.ErrNoKey {
				ck.mu.Lock()
				ck.leader = index
				ck.mu.Unlock()
				return "", 0, rpc.ErrNoKey
			}
			if ok && reply.Err == rpc.ErrWrongLeader {
				continue
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
	// Your code here
	ck.mu.Lock()
	requestId := ck.requestId
	ck.requestId++
	ck.mu.Unlock()
	args := &rpc.PutArgs{Key: key, Value: value, Version: version, ClientId: ck.clientId, RequestId: requestId}
	firstAttempt := true
	for {
		ck.mu.Lock()
		startLeader := ck.leader
		ck.mu.Unlock()

		for i := 0; i < len(ck.servers); i++ {
			reply := &rpc.PutReply{}
			index := (startLeader + i) % len(ck.servers)
			server := ck.servers[index]
			ok := ck.clnt.Call(server, "KVServer.Put", args, reply)
			if !ok {
				continue
			}

			switch reply.Err {
			case rpc.OK:
				ck.mu.Lock()
				ck.leader = index
				ck.mu.Unlock()
				return rpc.OK
			case rpc.ErrVersion:
				ck.mu.Lock()
				ck.leader = index
				ck.mu.Unlock()
				if !firstAttempt {
					return rpc.ErrMaybe
				}
				return rpc.ErrVersion
			case rpc.ErrNoKey:
				ck.mu.Lock()
				ck.leader = index
				ck.mu.Unlock()
				return rpc.ErrNoKey
			case rpc.ErrWrongLeader:
				continue
			}
		}
		firstAttempt = false
		time.Sleep(100 * time.Millisecond)
	}
}

func (ck *Clerk) FreezeShard(s shardcfg.Tshid, num shardcfg.Tnum) ([]byte, rpc.Err) {
	// Your code here
	return nil, ""
}

func (ck *Clerk) InstallShard(s shardcfg.Tshid, state []byte, num shardcfg.Tnum) rpc.Err {
	// Your code here
	return ""
}

func (ck *Clerk) DeleteShard(s shardcfg.Tshid, num shardcfg.Tnum) rpc.Err {
	// Your code here
	return ""
}
