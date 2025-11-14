package kvraft

import (
	"bytes"
	"sync"
	"sync/atomic"

	"6.5840/kvraft1/rsm"
	"6.5840/kvsrv1/rpc"
	"6.5840/labgob"
	"6.5840/labrpc"
	tester "6.5840/tester1"
)

type Entry struct {
	Value   string
	Version rpc.Tversion
}

type Session struct {
	ClientId  int
	RequestId int
	Reply     any
}

type KVServer struct {
	me   int
	dead int32 // set by Kill()
	rsm  *rsm.RSM

	// Your definitions here.
	mu       sync.Mutex
	data     map[string]*Entry
	sessions map[int]Session
}

// To type-cast req to the right type, take a look at Go's type switches or type
// assertions below:
//
// https://go.dev/tour/methods/16
// https://go.dev/tour/methods/15
func (kv *KVServer) DoOp(req any) any {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	switch req := req.(type) {
	case *rpc.GetArgs:
		session, ok := kv.sessions[req.ClientId]
		if ok && session.RequestId == req.RequestId {
			return session.Reply
		}
		entry, ok := kv.data[req.Key]
		var reply *rpc.GetReply
		if !ok {
			reply = &rpc.GetReply{Value: "", Version: 0, Err: rpc.ErrNoKey}
		} else {
			reply = &rpc.GetReply{Value: entry.Value, Version: entry.Version, Err: rpc.OK}
		}
		kv.sessions[req.ClientId] = Session{ClientId: req.ClientId, RequestId: req.RequestId, Reply: reply}
		return reply
	case *rpc.PutArgs:
		session, ok := kv.sessions[req.ClientId]
		if ok && session.RequestId == req.RequestId {
			return session.Reply
		}
		entry, ok := kv.data[req.Key]
		var reply *rpc.PutReply
		if !ok {
			if req.Version != 0 {
				reply = &rpc.PutReply{Err: rpc.ErrVersion}
			} else {
				entry = &Entry{Value: req.Value, Version: req.Version + 1}
				kv.data[req.Key] = entry
				reply = &rpc.PutReply{Err: rpc.OK}
			}
		} else {
			if entry.Version != req.Version {
				reply = &rpc.PutReply{Err: rpc.ErrVersion}
			} else {
				entry.Value = req.Value
				entry.Version = req.Version + 1
				kv.data[req.Key] = entry
				reply = &rpc.PutReply{Err: rpc.OK}
			}
		}
		kv.sessions[req.ClientId] = Session{ClientId: req.ClientId, RequestId: req.RequestId, Reply: reply}
		return reply
	default:
		return nil
	}
}

func (kv *KVServer) Snapshot() []byte {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(kv.data)
	e.Encode(kv.sessions)
	return w.Bytes()
}

func (kv *KVServer) Restore(data []byte) {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var newData map[string]*Entry
	var newSessions map[int]Session
	if d.Decode(&newData) != nil || d.Decode(&newSessions) != nil {
		return
	}
	kv.data = newData
	kv.sessions = newSessions
}

func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	// Your code here. Use kv.rsm.Submit() to submit args
	// You can use go's type casts to turn the any return value
	// of Submit() into a GetReply: rep.(rpc.GetReply)
	err, result := kv.rsm.Submit(args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	getReply := result.(*rpc.GetReply)
	reply.Value = getReply.Value
	reply.Version = getReply.Version
	reply.Err = getReply.Err
}

func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	// Your code here. Use kv.rsm.Submit() to submit args
	// You can use go's type casts to turn the any return value
	// of Submit() into a PutReply: rep.(rpc.PutReply)
	err, result := kv.rsm.Submit(args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	putReply := result.(*rpc.PutReply)
	reply.Err = putReply.Err
}

// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// StartKVServer() and MakeRSM() must return quickly, so they should
// start goroutines for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, gid tester.Tgid, me int, persister *tester.Persister, maxraftstate int) []tester.IService {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(rsm.Op{})
	labgob.Register(&rpc.PutArgs{})
	labgob.Register(&rpc.GetArgs{})
	labgob.Register(&rpc.GetReply{})
	labgob.Register(&rpc.PutReply{})
	labgob.Register(Entry{})
	labgob.Register(Session{})

	kv := &KVServer{me: me, data: make(map[string]*Entry), sessions: make(map[int]Session)}

	kv.rsm = rsm.MakeRSM(servers, me, persister, maxraftstate, kv)
	// You may need initialization code here.
	return []tester.IService{kv, kv.rsm.Raft()}
}
