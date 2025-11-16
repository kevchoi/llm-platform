package shardctrler

//
// Shardctrler with InitConfig, Query, and ChangeConfigTo methods
//

import (
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
}

// Make a ShardCltler, which stores its state in a kvsrv.
func MakeShardCtrler(clnt *tester.Clnt) *ShardCtrler {
	sck := &ShardCtrler{clnt: clnt}
	srv := tester.ServerName(tester.GRP0, 0)
	sck.IKVClerk = kvsrv.MakeClerk(clnt, srv)
	// Your code here.
	return sck
}

// The tester calls InitController() before starting a new
// controller. In part A, this method doesn't need to do anything. In
// B and C, this method implements recovery.
func (sck *ShardCtrler) InitController() {
	nextCfgStr, _, err := sck.IKVClerk.Get("conf-next")
	if err != rpc.OK || nextCfgStr == "" {
		return
	}
	nextCfg := shardcfg.FromString(nextCfgStr)
	if nextCfg == nil {
		return
	}
	currCfg := sck.Query()
	if nextCfg.Num > currCfg.Num {
		sck.moveShards(currCfg, nextCfg)
	}
}

// Called once by the tester to supply the first configuration.  You
// can marshal ShardConfig into a string using shardcfg.String(), and
// then Put it in the kvsrv for the controller at version 0.  You can
// pick the key to name the configuration.  The initial configuration
// lists shardgrp shardcfg.Gid1 for all shards.
func (sck *ShardCtrler) InitConfig(cfg *shardcfg.ShardConfig) {
	// Your code here
	sck.IKVClerk.Put("conf", cfg.String(), 0)
}

// Called by the tester to ask the controller to change the
// configuration from the current one to new.  While the controller
// changes the configuration it may be superseded by another
// controller.
func (sck *ShardCtrler) ChangeConfigTo(new *shardcfg.ShardConfig) {
	// Your code here.
	old := sck.Query()
	_, nextVer, _ := sck.IKVClerk.Get("conf-next")

	sck.IKVClerk.Put("conf-next", new.String(), nextVer)
	sck.moveShards(old, new)
}

func (sck *ShardCtrler) moveShards(old *shardcfg.ShardConfig, new *shardcfg.ShardConfig) {
	for shard := shardcfg.Tshid(0); shard < shardcfg.NShards; shard++ {
		oldGid := old.Shards[shard]
		newGid := new.Shards[shard]
		if oldGid == newGid {
			continue
		}
		if oldGid == 0 && newGid != 0 {
			_, newServers, _ := new.GidServers(shard)
			newClerk := shardgrp.MakeClerk(sck.clnt, newServers)
			newClerk.InstallShard(shard, []byte{}, new.Num)
			continue
		}
		if oldGid != 0 && newGid == 0 {
			_, oldServers, _ := old.GidServers(shard)
			oldClerk := shardgrp.MakeClerk(sck.clnt, oldServers)
			_, err := oldClerk.FreezeShard(shard, new.Num)
			if err == rpc.OK {
				oldClerk.DeleteShard(shard, new.Num)
			}
			continue
		}
		if oldGid != 0 && newGid != 0 {
			_, oldServers, _ := old.GidServers(shard)
			_, newServers, _ := new.GidServers(shard)

			oldClerk := shardgrp.MakeClerk(sck.clnt, oldServers)
			state, err := oldClerk.FreezeShard(shard, new.Num)
			if err != rpc.OK {
				continue
			}

			newClerk := shardgrp.MakeClerk(sck.clnt, newServers)
			err = newClerk.InstallShard(shard, state, new.Num)
			if err != rpc.OK {
				continue
			}
		}
	}

	// Update configuration before deleting shards so that clients can get the new configuration
	_, ver, _ := sck.IKVClerk.Get("conf")
	sck.IKVClerk.Put("conf", new.String(), ver)

	for shard := shardcfg.Tshid(0); shard < shardcfg.NShards; shard++ {
		oldGid := old.Shards[shard]
		newGid := new.Shards[shard]
		if oldGid != newGid && oldGid != 0 && newGid != 0 {
			_, oldServers, _ := old.GidServers(shard)
			oldClerk := shardgrp.MakeClerk(sck.clnt, oldServers)
			oldClerk.DeleteShard(shard, new.Num)
		}
	}

	_, ver, _ = sck.IKVClerk.Get("conf-next")
	sck.IKVClerk.Put("conf-next", "", ver)
}

// Return the current configuration
func (sck *ShardCtrler) Query() *shardcfg.ShardConfig {
	// Your code here.
	cfg, _, err := sck.IKVClerk.Get("conf")
	if err != rpc.OK {
		return nil
	}
	return shardcfg.FromString(cfg)
}
