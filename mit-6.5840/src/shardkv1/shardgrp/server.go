package shardgrp

import (
	"bytes"
	"log"
	"sync"
	"sync/atomic"

	"6.5840/kvraft1/rsm"
	"6.5840/kvsrv1/rpc"
	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/shardkv1/shardcfg"
	"6.5840/shardkv1/shardgrp/shardrpc"
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

type ShardState struct {
	FreezeNum  shardcfg.Tnum
	InstallNum shardcfg.Tnum
	DeleteNum  shardcfg.Tnum
	Owned      bool
	Frozen     bool
}

type KVServer struct {
	me   int
	dead int32 // set by Kill()
	rsm  *rsm.RSM
	gid  tester.Tgid

	// Your code here
	mu           sync.Mutex
	data         map[string]*Entry
	sessions     map[int]Session
	shards       map[shardcfg.Tshid]*ShardState
	frozenShards map[shardcfg.Tshid][]byte
}

func (kv *KVServer) DoOp(req any) any {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	switch req := req.(type) {
	case *rpc.GetArgs:
		shard := shardcfg.Key2Shard(req.Key)
		state, ok := kv.shards[shard]
		if !ok || !state.Owned {
			return &rpc.GetReply{Err: rpc.ErrWrongGroup}
		}
		entry, ok := kv.data[req.Key]
		var reply *rpc.GetReply
		if !ok {
			reply = &rpc.GetReply{Value: "", Version: 0, Err: rpc.ErrNoKey}
		} else {
			reply = &rpc.GetReply{Value: entry.Value, Version: entry.Version, Err: rpc.OK}
		}
		return reply
	case *rpc.PutArgs:
		shard := shardcfg.Key2Shard(req.Key)
		state, ok := kv.shards[shard]
		if !ok || !state.Owned || state.Frozen {
			return &rpc.PutReply{Err: rpc.ErrWrongGroup}
		}
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
	case *shardrpc.FreezeShardArgs:
		state, ok := kv.shards[req.Shard]
		if !ok || !state.Owned {
			return &shardrpc.FreezeShardReply{Err: rpc.ErrWrongGroup}
		}
		if req.Num <= state.FreezeNum {
			// Return the cached frozen state for duplicate requests
			cachedState, hasCached := kv.frozenShards[req.Shard]
			if !hasCached {
				cachedState = []byte{}
			}
			return &shardrpc.FreezeShardReply{
				Err:   rpc.OK,
				Num:   state.FreezeNum,
				State: cachedState,
			}
		}
		state.FreezeNum = req.Num
		state.Frozen = true
		shardData := make(map[string]*Entry)
		for key, entry := range kv.data {
			if shardcfg.Key2Shard(key) == req.Shard {
				shardData[key] = &Entry{Value: entry.Value, Version: entry.Version}
			}
		}
		w := new(bytes.Buffer)
		e := labgob.NewEncoder(w)
		e.Encode(shardData)
		frozenShard := w.Bytes()
		kv.frozenShards[req.Shard] = frozenShard
		return &shardrpc.FreezeShardReply{Err: rpc.OK, Num: state.FreezeNum, State: frozenShard}
	case *shardrpc.InstallShardArgs:
		state, ok := kv.shards[req.Shard]
		if !ok {
			state = &ShardState{Owned: false, Frozen: false, InstallNum: 0}
			kv.shards[req.Shard] = state
		}
		if req.Num <= state.InstallNum {
			state.Owned = true
			state.Frozen = false
			return &shardrpc.InstallShardReply{Err: rpc.OK}
		}
		state.InstallNum = req.Num
		if len(req.State) > 0 {
			r := bytes.NewBuffer(req.State)
			d := labgob.NewDecoder(r)
			var shardData map[string]*Entry
			if d.Decode(&shardData) != nil {
				log.Fatalf("%v couldn't decode shardData", shardData)
			}
			for key, entry := range shardData {
				kv.data[key] = &Entry{Value: entry.Value, Version: entry.Version}
			}
		}
		state.Owned = true
		state.Frozen = false
		return &shardrpc.InstallShardReply{Err: rpc.OK}
	case *shardrpc.DeleteShardArgs:
		state, ok := kv.shards[req.Shard]
		if !ok {
			return &shardrpc.DeleteShardReply{Err: rpc.ErrWrongGroup}
		}
		if req.Num <= state.DeleteNum {
			return &shardrpc.DeleteShardReply{Err: rpc.OK}
		}
		state.DeleteNum = req.Num
		for key := range kv.data {
			if shardcfg.Key2Shard(key) == req.Shard {
				delete(kv.data, key)
			}
		}
		delete(kv.shards, req.Shard)
		delete(kv.frozenShards, req.Shard)
		return &shardrpc.DeleteShardReply{Err: rpc.OK}
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
	e.Encode(kv.shards)
	e.Encode(kv.frozenShards)
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
	var newShards map[shardcfg.Tshid]*ShardState
	var newfrozenShards map[shardcfg.Tshid][]byte
	if d.Decode(&newData) != nil || d.Decode(&newSessions) != nil || d.Decode(&newShards) != nil || d.Decode(&newfrozenShards) != nil {
		return
	}
	kv.data = newData
	kv.sessions = newSessions
	kv.shards = newShards
	kv.frozenShards = newfrozenShards
}

func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	// Your code here
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
	// Your code here
	err, result := kv.rsm.Submit(args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	putReply := result.(*rpc.PutReply)
	reply.Err = putReply.Err
}

// Freeze the specified shard (i.e., reject future Get/Puts for this
// shard) and return the key/values stored in that shard.
func (kv *KVServer) FreezeShard(args *shardrpc.FreezeShardArgs, reply *shardrpc.FreezeShardReply) {
	// Your code here
	err, result := kv.rsm.Submit(args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	freezeReply := result.(*shardrpc.FreezeShardReply)
	reply.State = freezeReply.State
	reply.Num = freezeReply.Num
	reply.Err = freezeReply.Err
}

// Install the supplied state for the specified shard.
func (kv *KVServer) InstallShard(args *shardrpc.InstallShardArgs, reply *shardrpc.InstallShardReply) {
	// Your code here
	err, result := kv.rsm.Submit(args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	installReply := result.(*shardrpc.InstallShardReply)
	reply.Err = installReply.Err
}

// Delete the specified shard.
func (kv *KVServer) DeleteShard(args *shardrpc.DeleteShardArgs, reply *shardrpc.DeleteShardReply) {
	// Your code here
	err, result := kv.rsm.Submit(args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	deleteReply := result.(*shardrpc.DeleteShardReply)
	reply.Err = deleteReply.Err
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

// StartShardServerGrp starts a server for shardgrp `gid`.
//
// StartShardServerGrp() and MakeRSM() must return quickly, so they should
// start goroutines for any long-running work.
func StartServerShardGrp(servers []*labrpc.ClientEnd, gid tester.Tgid, me int, persister *tester.Persister, maxraftstate int) []tester.IService {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall
	labgob.Register(&shardrpc.FreezeShardArgs{})
	labgob.Register(&shardrpc.InstallShardArgs{})
	labgob.Register(&shardrpc.DeleteShardArgs{})
	labgob.Register(&shardrpc.FreezeShardReply{})
	labgob.Register(&shardrpc.InstallShardReply{})
	labgob.Register(&shardrpc.DeleteShardReply{})
	labgob.Register(&ShardState{})
	labgob.Register(rsm.Op{})

	labgob.Register(&rpc.PutArgs{})
	labgob.Register(&rpc.GetArgs{})
	labgob.Register(&rpc.GetReply{})
	labgob.Register(&rpc.PutReply{})
	labgob.Register(Entry{})
	labgob.Register(Session{})

	kv := &KVServer{
		gid:          gid,
		me:           me,
		data:         make(map[string]*Entry),
		sessions:     make(map[int]Session),
		shards:       make(map[shardcfg.Tshid]*ShardState),
		frozenShards: make(map[shardcfg.Tshid][]byte),
	}
	kv.rsm = rsm.MakeRSM(servers, me, persister, maxraftstate, kv)

	// Your code here
	if gid == shardcfg.Gid1 {
		for i := shardcfg.Tshid(0); i < shardcfg.NShards; i++ {
			kv.shards[i] = &ShardState{
				Owned:      true,
				Frozen:     false,
				FreezeNum:  0,
				InstallNum: 0,
				DeleteNum:  0,
			}
		}
	}

	return []tester.IService{kv, kv.rsm.Raft()}
}
