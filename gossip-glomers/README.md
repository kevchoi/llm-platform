# Gossip Glomers

## Prerequisites

- Go
- [Maelstrom 0.2.3](https://github.com/jepsen-io/maelstrom/releases/tag/v0.2.3)
- `brew install openjdk graphviz gnuplot`

## Setup

```
$ mkdir maelstrom-$
$ cd maelstrom-echo
$ go mod init maelstrom-echo
$ go mod tidy
$ go get github.com/jepsen-io/maelstrom/demo/go
$ go install .
```

## Run in Maelstrom

```
./maelstrom test -w echo --bin ~/go/bin/maelstrom-echo --node-count 1 --time-limit 10
```

## Debugging

```
./maelstrom serve
```