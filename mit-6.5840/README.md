# MIT 6.5840 Distributed Systems

Implementations of the [MIT 6.5840](https://pdos.csail.mit.edu/6.824/) distributed systems labs.

## Labs

- **Lab 1: MapReduce** (`src/mr`)
- **Lab 2: Key/Value Server** (`src/kvsrv1`)
- **Lab 3: Raft Consensus** (`src/raft1`)
- **Lab 4: Fault-Tolerant KV** (`src/kvraft1`)
- **Lab 5: Sharded KV** (`src/shardkv1`)

## Usage

Run tests for a specific lab (e.g., Raft):

cd src/raft1
go test --race -v