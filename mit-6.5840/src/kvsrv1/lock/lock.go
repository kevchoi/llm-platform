package lock

import (
	"6.5840/kvsrv1/rpc"
	kvtest "6.5840/kvtest1"
)

type Lock struct {
	// IKVClerk is a go interface for k/v clerks: the interface hides
	// the specific Clerk type of ck but promises that ck supports
	// Put and Get.  The tester passes the clerk in when calling
	// MakeLock().
	ck kvtest.IKVClerk
	// You may add code here
	key     string
	version rpc.Tversion
}

// The tester calls MakeLock() and passes in a k/v clerk; your code can
// perform a Put or Get by calling lk.ck.Put() or lk.ck.Get().
//
// Use l as the key to store the "lock state" (you would have to decide
// precisely what the lock state is).
func MakeLock(ck kvtest.IKVClerk, l string) *Lock {
	lk := &Lock{ck: ck}
	// You may add code here
	lk.key = l
	return lk
}

func (lk *Lock) Acquire() {
	// Your code here
	for {
		value, version, err := lk.ck.Get(lk.key)
		if err == rpc.OK {
			if value == "acquired" {
				continue
			}
		} else if err == rpc.ErrNoKey {
			version = 0
		} else {
			continue
		}
		lk.version = version + 1
		ok := lk.ck.Put(lk.key, "acquired", version)
		if ok == rpc.OK {
			return
		} else if ok == rpc.ErrMaybe {
			value, version, err := lk.ck.Get(lk.key)
			if err == rpc.OK && value == "acquired" && version == lk.version {
				return
			}
		}
	}
}

func (lk *Lock) Release() {
	for {
		ok := lk.ck.Put(lk.key, "released", lk.version)
		if ok == rpc.OK {
			return
		} else if ok == rpc.ErrMaybe {
			value, _, err := lk.ck.Get(lk.key)
			if err == rpc.OK && value == "released" {
				return
			}
		}
	}
}
