package kvraft

import (
	"math/rand"
	"sync"
	"time"

	"6.5840/kvsrv1/rpc"
	kvtest "6.5840/kvtest1"
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

func MakeClerk(clnt *tester.Clnt, servers []string) kvtest.IKVClerk {
	ck := &Clerk{clnt: clnt, servers: servers, leader: rand.Intn(len(servers))}
	// You'll have to add code here.
	ck.clientId = rand.Int()
	ck.requestId = 0
	return ck
}

// Get fetches the current value and version for a key.  It returns
// ErrNoKey if the key does not exist. It keeps trying forever in the
// face of all other errors.
//
// You can send an RPC to server i with code like this:
// ok := ck.clnt.Call(ck.servers[i], "KVServer.Get", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	// You will have to modify this function.
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

// Put updates key with value only if the version in the
// request matches the version of the key at the server.  If the
// versions numbers don't match, the server should return
// ErrVersion.  If Put receives an ErrVersion on its first RPC, Put
// should return ErrVersion, since the Put was definitely not
// performed at the server. If the server returns ErrVersion on a
// resend RPC, then Put must return ErrMaybe to the application, since
// its earlier RPC might have been processed by the server successfully
// but the response was lost, and the the Clerk doesn't know if
// the Put was performed or not.
//
// You can send an RPC to server i with code like this:
// ok := ck.clnt.Call(ck.servers[i], "KVServer.Put", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
	// You will have to modify this function.
	ck.mu.Lock()
	requestId := ck.requestId
	ck.requestId++
	ck.mu.Unlock()
	args := &rpc.PutArgs{Key: key, Value: value, Version: version, ClientId: ck.clientId, RequestId: requestId}
	firstTry := true
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

			if reply.Err != rpc.ErrWrongLeader && firstTry {
				firstTry = false
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
				if firstTry {
					return rpc.ErrVersion
				}
				return rpc.ErrMaybe
			case rpc.ErrNoKey:
				ck.mu.Lock()
				ck.leader = index
				ck.mu.Unlock()
				return rpc.ErrNoKey
			case rpc.ErrWrongLeader:
				continue
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
