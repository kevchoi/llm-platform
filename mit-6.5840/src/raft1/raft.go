package raft

// The file raftapi/raft.go defines the interface that raft must
// expose to servers (or the tester), but see comments below for each
// of these functions for more details.
//
// Make() creates a new raft peer that implements the raft interface.

import (
	//	"bytes"

	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raftapi"
	tester "6.5840/tester1"
)

type Role int

const (
	Follower Role = iota
	Candidate
	Leader
)

type LogEntry struct {
	Term    int
	Command interface{}
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *tester.Persister   // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).

	// Helpers
	applyCh         chan raftapi.ApplyMsg
	role            Role
	electionTimeout time.Time

	// State: Persistent state on all servers (updated on stable storage before responding to RPCs)
	currentTerm int
	votedFor    int
	log         []LogEntry

	// State: Volatile state on all servers
	commitIndex int
	lastApplied int

	// State: Volatile state on leaders (reinitialized after election)
	nextIndex  []int
	matchIndex []int

	// State: Snapshot state
	lastIncludedIndex int
	lastIncludedTerm  int
	snapshot          []byte

	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
}

func (rf *Raft) getIndex(index int) int {
	return index - rf.lastIncludedIndex
}

func (rf *Raft) getLogLength() int {
	return len(rf.log) + rf.lastIncludedIndex
}

func (rf *Raft) getTerm(index int) int {
	return rf.log[rf.getIndex(index)].Term
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	var term int
	var isleader bool
	// Your code here (3A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = rf.role == Leader

	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (3C).
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)
	raftstate := w.Bytes()
	rf.persister.Save(raftstate, rf.snapshot)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (3C).
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)

	var currentTerm int
	var votedFor int
	var log []LogEntry
	var lastIncludedIndex int
	var lastIncludedTerm int
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&log) != nil ||
		d.Decode(&lastIncludedIndex) != nil ||
		d.Decode(&lastIncludedTerm) != nil {
		return
	}
	rf.currentTerm = currentTerm
	rf.votedFor = votedFor
	rf.log = log
	rf.lastIncludedIndex = lastIncludedIndex
	rf.lastIncludedTerm = lastIncludedTerm
	rf.snapshot = rf.persister.ReadSnapshot()
}

// how many bytes in Raft's persisted log?
func (rf *Raft) PersistBytes() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.persister.RaftStateSize()
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (3D).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if index <= rf.lastIncludedIndex {
		return
	}
	if index >= rf.getLogLength() {
		return
	}

	rf.lastIncludedTerm = rf.getTerm(index)
	newLog := []LogEntry{
		{Term: rf.lastIncludedTerm, Command: nil},
	}
	rf.log = append(newLog, rf.log[rf.getIndex(index)+1:]...)

	rf.lastIncludedIndex = index
	rf.snapshot = snapshot
	rf.persist()
}

type InstallSnapshotArgs struct {
	Term              int
	LeaderId          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte
	Done              bool
}

type InstallSnapshotReply struct {
	Term int
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	// Return false if term < currentTerm (RequestVote RPC: Receiver implementation #1)
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		rf.mu.Unlock()
		return
	}

	// Convert to follower if term > currentTerm (Rules for Servers: All Servers #2)
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = Follower
		rf.votedFor = -1
		rf.persist()
	}
	rf.electionTimeout = getElectionTimeout()
	reply.Term = rf.currentTerm

	if args.LastIncludedIndex <= rf.lastIncludedIndex {
		rf.mu.Unlock()
		return
	}

	// Save snapshot file, discard any existing or partial snapshot with a smaller index (InstallSnapshot RPC: Receiver implementation)
	rf.snapshot = args.Data

	newLog := []LogEntry{
		{Term: args.LastIncludedTerm, Command: nil},
	}

	// If existing log entry has same index and term as snapshot’s last included entry, retain log entries following it and reply (InstallSnapshot RPC: Receiver implementation)
	if args.LastIncludedIndex < rf.getLogLength() && rf.getTerm(args.LastIncludedIndex) == args.LastIncludedTerm {
		rf.log = append(newLog, rf.log[rf.getIndex(args.LastIncludedIndex)+1:]...)
	} else {
		// Discard the entire log (InstallSnapshot RPC: Receiver implementation)
		rf.log = newLog
	}

	rf.lastIncludedIndex = args.LastIncludedIndex
	rf.lastIncludedTerm = args.LastIncludedTerm
	rf.commitIndex = max(args.LastIncludedIndex, rf.commitIndex)
	rf.lastApplied = max(args.LastIncludedIndex, rf.lastApplied)

	rf.persist()

	rf.mu.Unlock()
	// Reset state machine using snapshot contents (and load snapshot’s cluster configuration) (InstallSnapshot RPC: Receiver implementation)
	applyMsg := raftapi.ApplyMsg{
		Snapshot:      args.Data,
		SnapshotValid: true,
		SnapshotTerm:  args.LastIncludedTerm,
		SnapshotIndex: args.LastIncludedIndex,
	}
	rf.applyCh <- applyMsg
}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (3A, 3B).
	Term         int
	CandidateID  int
	LastLogIndex int
	LastLogTerm  int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (3A).
	Term        int
	VoteGranted bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Return false if term < currentTerm (RequestVote RPC: Receiver implementation #1)
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// Convert to follower if term > currentTerm (Rules for Servers: All Servers #2)
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = Follower
		rf.votedFor = -1
		rf.electionTimeout = getElectionTimeout()
		rf.persist()
	}

	reply.Term = rf.currentTerm
	reply.VoteGranted = false
	// Grant vote if votedFor is -1 or candidateId... (RequestVote RPC: Receiver implementation #2)
	if rf.votedFor == -1 || rf.votedFor == args.CandidateID {
		lastLogIndex := rf.getLogLength() - 1
		lastLogTerm := rf.getTerm(lastLogIndex)
		// ... and candidate's log is at least as up-to-date as receiver's log (RequestVote RPC: Receiver implementation #2)
		//   "If the logs have last entries with different terms, then the log with the later term is more up-to-date.
		//    If the logs end with the same term, then whichever log is longer is more up-to-date."
		if args.LastLogTerm > lastLogTerm || (args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex) {
			rf.votedFor = args.CandidateID
			reply.VoteGranted = true
			rf.electionTimeout = getElectionTimeout()
			rf.persist()
		}
	}
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
	XTerm   int
	XIndex  int
	XLen    int
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Return false if term < currentTerm (AppendEntries RPC: Receiver implementation #1)
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// Convert to follower if term > currentTerm (Rules for Servers: All Servers #2)
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = Follower
		rf.votedFor = -1
		rf.persist()
	}
	// Reset election timeout
	rf.electionTimeout = getElectionTimeout()

	// Return false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm (AppendEntries RPC: Receiver implementation #2)
	if args.PrevLogIndex >= rf.getLogLength() || args.PrevLogIndex < rf.lastIncludedIndex {
		reply.Term = rf.currentTerm
		reply.Success = false
		reply.XTerm = -1
		reply.XIndex = -1
		reply.XLen = rf.getLogLength()
		return
	}
	if rf.getTerm(args.PrevLogIndex) != args.PrevLogTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		reply.XTerm = rf.getTerm(args.PrevLogIndex)
		reply.XIndex = rf.lastIncludedIndex + 1
		for i := args.PrevLogIndex; i > rf.lastIncludedIndex; i-- {
			if rf.getTerm(i) != reply.XTerm {
				reply.XIndex = i + 1
				break
			}
		}
		reply.XLen = rf.getLogLength()
		return
	}

	for i, entry := range args.Entries {
		currIndex := args.PrevLogIndex + 1 + i
		if currIndex < rf.getLogLength() {
			// If an existing entry conflicts with a new one (same index but different terms), delete the existing entry and all that follow it (AppendEntries RPC: Receiver implementation #3)
			if rf.getTerm(currIndex) != entry.Term {
				rf.log = rf.log[:rf.getIndex(currIndex)]
				rf.log = append(rf.log, args.Entries[i:]...)
				rf.persist()
				break
			}
		} else {
			// Append any new entries not already in the log (AppendEntries RPC: Receiver implementation #3)
			rf.log = append(rf.log, args.Entries[i:]...)
			rf.persist()
			break
		}
	}

	// If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry) (AppendEntries RPC: Receiver implementation #5)
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.getLogLength()-1)
	}

	reply.Term = rf.currentTerm
	reply.Success = true
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	// Your code here (3B).
	rf.mu.Lock()
	if rf.role != Leader {
		defer rf.mu.Unlock()
		return -1, rf.currentTerm, false
	}

	// If command received from client: append entry to local log, respond after entry applied to state machine (Rules for Servers: Leaders #2)
	entry := LogEntry{
		Term:    rf.currentTerm,
		Command: command,
	}
	rf.log = append(rf.log, entry)
	rf.persist()

	index := rf.getLogLength() - 1
	term := rf.currentTerm

	rf.mu.Unlock()
	go rf.sendAppendEntriesToAllPeers()

	return index, term, true
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func getElectionTimeout() time.Time {
	ms := 200 + (rand.Int63() % 200)
	return time.Now().Add(time.Duration(ms) * time.Millisecond)
}

func (rf *Raft) startElection() {
	// On conversion to candidate, start election: (Rules for Servers: Candidates #1)
	// - Increment currentTerm
	// - Vote for self
	// - Reset election timeout
	// - Send RequestVote RPCs to all other servers
	rf.mu.Lock()
	rf.role = Candidate
	rf.currentTerm += 1
	rf.votedFor = rf.me
	rf.electionTimeout = getElectionTimeout()
	rf.persist()

	currentTerm := rf.currentTerm
	lastLogIndex := rf.getLogLength() - 1
	lastLogTerm := rf.getTerm(lastLogIndex)
	candidateID := rf.me
	numPeers := len(rf.peers)
	rf.mu.Unlock()

	votesCh := make(chan int, numPeers-1)
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(server int) {
			args := RequestVoteArgs{
				Term:         currentTerm,
				CandidateID:  candidateID,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}
			reply := RequestVoteReply{}
			ok := rf.sendRequestVote(server, &args, &reply)
			if ok && reply.VoteGranted {
				votesCh <- 1
			} else {
				votesCh <- 0
			}
			if ok {
				rf.mu.Lock()
				defer rf.mu.Unlock()
				// If RPC request or response contains term T > currentTerm, set currentTerm to T and convert to follower (Rules for Servers: All Servers #2)
				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.role = Follower
					rf.votedFor = -1
					rf.electionTimeout = getElectionTimeout()
					rf.persist()
				}
			}
		}(i)
	}
	go func() {
		votes := 1
		votesNeeded := numPeers/2 + 1
		for range numPeers - 1 {
			votes += <-votesCh
			// If votes received from majority of servers: become leader (Rules for Servers: Candidates #2)
			if votes >= votesNeeded {
				rf.mu.Lock()
				if rf.role == Candidate && rf.currentTerm == currentTerm {
					rf.role = Leader
					// Reinitialized after election
					rf.nextIndex = make([]int, len(rf.peers))
					rf.matchIndex = make([]int, len(rf.peers))
					for i := range rf.peers {
						rf.nextIndex[i] = rf.getLogLength()
						rf.matchIndex[i] = rf.lastIncludedIndex
					}
					rf.mu.Unlock()
					// Upon election: send initial empty AppendEntries RPCs (heartbeat) to each server; repeat during idle periods toprevent election timeouts (Rules for Servers: Leaders #1)
					go rf.sendAppendEntriesToAllPeers()
					return
				}
				rf.mu.Unlock()
				return
			}
		}
	}()
}

func (rf *Raft) sendAppendEntriesToAllPeers() {
	rf.mu.Lock()
	if rf.role != Leader {
		rf.mu.Unlock()
		return
	}
	leaderId := rf.me
	peers := rf.peers
	rf.mu.Unlock()
	for i := range peers {
		if i == leaderId {
			continue
		}
		go func(server int) {
			rf.mu.Lock()
			prevLogIndex := rf.nextIndex[server] - 1
			if prevLogIndex < rf.lastIncludedIndex {
				args := InstallSnapshotArgs{
					Term:              rf.currentTerm,
					LeaderId:          rf.me,
					LastIncludedIndex: rf.lastIncludedIndex,
					LastIncludedTerm:  rf.lastIncludedTerm,
					Data:              rf.snapshot,
				}
				rf.mu.Unlock()

				reply := InstallSnapshotReply{}
				ok := rf.sendInstallSnapshot(server, &args, &reply)
				if ok {
					rf.mu.Lock()
					defer rf.mu.Unlock()
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.role = Follower
						rf.votedFor = -1
						rf.electionTimeout = getElectionTimeout()
						rf.persist()
					}
					rf.nextIndex[server] = args.LastIncludedIndex + 1
					rf.matchIndex[server] = args.LastIncludedIndex
				}
			} else {
				args := AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: prevLogIndex,
					PrevLogTerm:  rf.getTerm(prevLogIndex),
					Entries:      rf.log[rf.getIndex(prevLogIndex)+1:],
					LeaderCommit: rf.commitIndex,
				}
				rf.mu.Unlock()

				reply := AppendEntriesReply{}
				ok := rf.sendAppendEntries(server, &args, &reply)

				if ok {
					rf.mu.Lock()
					defer rf.mu.Unlock()
					// If AppendEntries RPC received from new leader: convert to follower (Rules for Servers: Candidates #3)
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.role = Follower
						rf.votedFor = -1
						rf.electionTimeout = getElectionTimeout()
						rf.persist()
						return
					}
					if reply.Term != rf.currentTerm {
						return
					}
					// If last log index >= nextIndex for a follower: sendAppendEntries RPC with log entries starting at nextIndex. (Rules for Servers: Leaders #3)
					// - If successful: update nextIndex and matchIndex for follower
					// - If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
					if reply.Success {
						rf.nextIndex[server] = args.PrevLogIndex + 1 + len(args.Entries)
						rf.matchIndex[server] = args.PrevLogIndex + len(args.Entries)
						// If there exists an N such that N > commitIndex, a majority of matchIndex[i] >= N, and log[N].term == currentTerm: set commitIndex = N (Rules for Servers: Leaders #4)
						for n := rf.commitIndex + 1; n < rf.getLogLength(); n++ {
							if rf.getTerm(n) != rf.currentTerm {
								continue
							}
							count := 1
							for i := range rf.peers {
								if i != rf.me && rf.matchIndex[i] >= n {
									count++
								}
							}
							if count > len(rf.peers)/2 {
								rf.commitIndex = n
							}
						}
					} else {
						if reply.XTerm == -1 {
							rf.nextIndex[server] = reply.XLen
						} else {
							rf.nextIndex[server] = reply.XIndex
							for j := rf.getLogLength() - 1; j >= rf.lastIncludedIndex; j-- {
								if rf.getTerm(j) == reply.XTerm {
									rf.nextIndex[server] = j + 1
									break
								}
								if rf.getTerm(j) < reply.XTerm {
									break
								}
							}

						}
					}
				}
			}
		}(i)
	}
}

func (rf *Raft) electionTicker() {
	for !rf.killed() {
		rf.mu.Lock()
		role := rf.role
		electionTimeout := rf.electionTimeout
		rf.mu.Unlock()

		// If election timeout lapses without receiving AppendEntries RPC from current leader or granting vote to candidate: convert to candidate (Rules for Servers: Followers #2)
		// If election timeout elapses: start new election (Rules for Servers: Candidates #4)
		if role != Leader && time.Now().After(electionTimeout) {
			rf.startElection()
		}
		ms := 50 + (rand.Int63() % 50)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

func (rf *Raft) leaderTicker() {
	for !rf.killed() {
		rf.mu.Lock()
		role := rf.role
		rf.mu.Unlock()
		if role == Leader {
			// Upon election: send initial empty AppendEntries RPCs (heartbeat) to each server; repeat during idle periods toprevent election timeouts (Rules for Servers: Leaders #1)
			rf.sendAppendEntriesToAllPeers()
			rf.mu.Lock()
			// If there exists an N such that N > commitIndex, a majority of matchIndex[i] >= N, and log[N].term == currentTerm: set commitIndex = N (Rules for Servers: Leaders #4)
			for n := rf.commitIndex + 1; n < rf.getLogLength(); n++ {
				if rf.getTerm(n) != rf.currentTerm {
					continue
				}
				count := 1
				for i := range rf.peers {
					if i != rf.me && rf.matchIndex[i] >= n {
						count++
					}
				}
				if count > len(rf.peers)/2 {
					rf.commitIndex = n
				}
			}
			rf.mu.Unlock()
		}
		time.Sleep(time.Duration(100) * time.Millisecond)
	}
}

func (rf *Raft) commitTicker() {
	for !rf.killed() {
		messagesToApply := make([]raftapi.ApplyMsg, 0)
		rf.mu.Lock()
		// If commitIndex > lastApplied: increment lastApplied, apply log[lastApplied] to state machine (Rules for Servers: All Servers #1)
		for rf.commitIndex > rf.lastApplied {
			rf.lastApplied += 1
			applyMsg := raftapi.ApplyMsg{
				CommandValid: true,
				Command:      rf.log[rf.getIndex(rf.lastApplied)].Command,
				CommandIndex: rf.lastApplied,
			}
			messagesToApply = append(messagesToApply, applyMsg)
		}
		rf.mu.Unlock()
		for _, applyMsg := range messagesToApply {
			rf.applyCh <- applyMsg
		}
		time.Sleep(time.Duration(10) * time.Millisecond)
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *tester.Persister, applyCh chan raftapi.ApplyMsg) raftapi.Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (3A, 3B, 3C).
	rf.applyCh = applyCh
	rf.role = Follower
	rf.electionTimeout = getElectionTimeout()

	rf.currentTerm = 0
	rf.votedFor = -1
	// Create dummy entry at index 0
	rf.log = []LogEntry{
		{Term: 0, Command: nil},
	}

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.lastIncludedIndex = 0
	rf.lastIncludedTerm = 0

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	rf.commitIndex = rf.lastIncludedIndex
	rf.lastApplied = rf.lastIncludedIndex

	// Start ticker goroutines for elections, leadership, and commits
	go rf.electionTicker()
	go rf.leaderTicker()
	go rf.commitTicker()

	if len(rf.snapshot) > 0 {
		go func() {
			rf.applyCh <- raftapi.ApplyMsg{
				Snapshot:      rf.snapshot,
				SnapshotValid: true,
				SnapshotTerm:  rf.lastIncludedTerm,
				SnapshotIndex: rf.lastIncludedIndex,
			}
		}()
	}

	return rf
}
