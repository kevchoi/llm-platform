package shardctrler

//
// Shardctrler with InitConfig, Query, and ChangeConfigTo methods
//

import (
	"fmt"
	"time"

	kvsrv "6.5840/kvsrv1"
	"6.5840/kvsrv1/rpc"
	kvtest "6.5840/kvtest1"
	"6.5840/shardkv1/shardcfg"
	"6.5840/shardkv1/shardgrp"
	tester "6.5840/tester1"
)

// ShardCtrler for the controller and kv clerk.
type ShardCtrler struct {
	clnt *tester.Clnt
	kvtest.IKVClerk

	killed int32 // set by Kill()

	// Your data here.
	id string // for debugging
}

// Make a ShardCltler, which stores its state in a kvsrv.
func MakeShardCtrler(clnt *tester.Clnt) *ShardCtrler {
	sck := &ShardCtrler{clnt: clnt, id: fmt.Sprintf("CTL-%d", time.Now().UnixNano()%10000)}
	srv := tester.ServerName(tester.GRP0, 0)
	sck.IKVClerk = kvsrv.MakeClerk(clnt, srv)
	// Your code here.
	return sck
}

// The tester calls InitController() before starting a new
// controller. In part A, this method doesn't need to do anything. In
// B and C, this method implements recovery.
func (sck *ShardCtrler) InitController() {
	currCfgStr, _, _ := sck.IKVClerk.Get("conf")
	nextCfgStr, _, _ := sck.IKVClerk.Get("conf-next")
	if nextCfgStr == "" {
		return
	}
	currCfg := shardcfg.FromString(currCfgStr)
	nextCfg := shardcfg.FromString(nextCfgStr)
	if nextCfg.Num <= currCfg.Num {
		_, nextVer, _ := sck.IKVClerk.Get("conf-next")
		sck.IKVClerk.Put("conf-next", "", nextVer)
		return
	}
	sck.moveShards(currCfg, nextCfg)

	// Re-check if another controller already updated conf while we were doing moveShards
	currCfgStr, currVer, _ := sck.IKVClerk.Get("conf")
	currCfg = shardcfg.FromString(currCfgStr)
	if currCfg.Num >= nextCfg.Num {
		_, nextVer, _ := sck.IKVClerk.Get("conf-next")
		sck.IKVClerk.Put("conf-next", "", nextVer)
		return
	}

	err := sck.IKVClerk.Put("conf", nextCfg.String(), currVer)
	if err != rpc.OK {
		return
	}
	_, nextVer, _ := sck.IKVClerk.Get("conf-next")
	sck.IKVClerk.Put("conf-next", "", nextVer)
}

// Called once by the tester to supply the first configuration.  You
// can marshal ShardConfig into a string using shardcfg.String(), and
// then Put it in the kvsrv for the controller at version 0.  You can
// pick the key to name the configuration.  The initial configuration
// lists shardgrp shardcfg.Gid1 for all shards.
func (sck *ShardCtrler) InitConfig(cfg *shardcfg.ShardConfig) {
	// Your code here
	sck.IKVClerk.Put("conf", cfg.String(), 0)
	sck.IKVClerk.Put("conf-next", "", 0)
}

// Called by the tester to ask the controller to change the
// configuration from the current one to new.  While the controller
// changes the configuration it may be superseded by another
// controller.
func (sck *ShardCtrler) ChangeConfigTo(new *shardcfg.ShardConfig) {
	// Your code here.
	nextCfgStr, nextVer, _ := sck.IKVClerk.Get("conf-next")
	if nextCfgStr != "" {
		nextCfg := shardcfg.FromString(nextCfgStr)
		if nextCfg != nil && new.Num <= nextCfg.Num {
			return
		}
	}

	err := sck.IKVClerk.Put("conf-next", new.String(), nextVer)
	if err != rpc.OK && err != rpc.ErrMaybe {
		return
	}

	currCfgStr, _, _ := sck.IKVClerk.Get("conf")
	currCfg := shardcfg.FromString(currCfgStr)
	if new.Num <= currCfg.Num {
		_, nextVer, _ := sck.IKVClerk.Get("conf-next")
		sck.IKVClerk.Put("conf-next", "", nextVer)
		return
	}
	sck.moveShards(currCfg, new)

	// Re-check if another controller already updated conf while we were doing moveShards
	currCfgStr, currVer, _ := sck.IKVClerk.Get("conf")
	currCfg = shardcfg.FromString(currCfgStr)
	if currCfg.Num >= new.Num {
		_, nextVer, _ := sck.IKVClerk.Get("conf-next")
		sck.IKVClerk.Put("conf-next", "", nextVer)
		return
	}

	err = sck.IKVClerk.Put("conf", new.String(), currVer)

	// Always try to clean up conf-next, regardless of whether our Put succeeded
	_, nextVer, _ = sck.IKVClerk.Get("conf-next")
	sck.IKVClerk.Put("conf-next", "", nextVer)
}

func (sck *ShardCtrler) moveShards(old *shardcfg.ShardConfig, new *shardcfg.ShardConfig) {
	for i, oldGid := range old.Shards {
		newGid := new.Shards[i]
		if oldGid == newGid {
			continue
		}

		oldServers := old.Groups[oldGid]
		oldClerk := shardgrp.MakeClerk(sck.clnt, oldServers)
		var state []byte
		var err rpc.Err
		for {
			state, err = oldClerk.FreezeShard(shardcfg.Tshid(i), new.Num)
			if err == rpc.OK || err == rpc.ErrWrongGroup {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if err == rpc.ErrWrongGroup {
			continue
		}

		newServers := new.Groups[newGid]
		newClerk := shardgrp.MakeClerk(sck.clnt, newServers)
		for {
			err = newClerk.InstallShard(shardcfg.Tshid(i), state, new.Num)
			if err == rpc.OK {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		for {
			err = oldClerk.DeleteShard(shardcfg.Tshid(i), new.Num)
			if err == rpc.OK || err == rpc.ErrWrongGroup {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// Return the current configuration
func (sck *ShardCtrler) Query() *shardcfg.ShardConfig {
	// Your code here.
	for {
		val, _, err := sck.IKVClerk.Get("conf")
		if err == rpc.OK {
			return shardcfg.FromString(val)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
