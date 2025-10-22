package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"fmt"
	"labrpc"
	"math/rand"
	"os"
	"sync"
	"time"
)

// import "bytes"
// import "encoding/gob"

type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

type LogEntry struct {
	Term    int
	Command interface{}
}

// -------------------------------------- HELPERS START --------------------------------------

// defining the states a node can be
type state string

const (
	leader             state = "leader"
	candidate          state = "candidate"
	follower           state = "follower"
	minElectionTimeout       = 500 * time.Millisecond
	maxElectionTimeout       = 900 * time.Millisecond
	heartbeatInterval        = 100 * time.Millisecond
)

func (rf *Raft) say(format string, a ...interface{}) {
	// prefix := fmt.Sprintf("[%d]: ", rf.me)
	// fmt.Printf(prefix+format+"\n", a...)
	prefix := fmt.Sprintf("[%d]: ", rf.me)
	line := fmt.Sprintf(prefix+format+"\n", a...)

	// open file for append-only write
	f, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return // silently ignore logging errors
	}
	defer f.Close()

	// append the line
	f.WriteString(line)

}

func (rf *Raft) resetElectionTimeoutLocked(by string) {
	rf.electionTimeout = time.Duration(rand.Int63n(int64(maxElectionTimeout-minElectionTimeout))) + minElectionTimeout
	rf.say("just reset my election timeout by (%s)", by)
}

func (rf *Raft) updateLastHeartbeatLocked(by string) {
	rf.lastHeartbeat = time.Now()
	rf.say("just reset my heartbeat by (%s)", by)
}

func (rf *Raft) stepDownLocked(term int) {

	if term > rf.term {
		rf.term = term
	}
	rf.votedFor = -1
	rf.state = follower
	rf.voteCount = 0
	rf.updateLastHeartbeatLocked("stepDown")
	rf.resetElectionTimeoutLocked("stepDown")
	//rf.say("stepped down to follower, term=%d", rf.term)
}

func (rf *Raft) becomeLeaderLocked() {
	rf.say("I am now a leader in term #%d (won with %d votes) YAY !!!!!!!!!!!!!!!!", rf.term, rf.voteCount)
	rf.state = leader

	// log stuff
	n := len(rf.peers)
	rf.nextIndex = make([]int, n)
	rf.matchIndex = make([]int, n)

	last := rf.lastLogIndexLocked() //  last is 0 on a fresh node
	for i := 0; i < n; i++ {
		rf.nextIndex[i] = last + 1 // start by trying to append after last
		rf.matchIndex[i] = 0
	}
	rf.matchIndex[rf.me] = last // match up to what we actually have
	rf.nextIndex[rf.me] = last + 1

	// immediate round of empty AppendEntries to assert dominance lmao
	term := rf.term
	me := rf.me
	go func() {
		for p := 0; p < n; p++ {
			if p == me {
				continue
			}
			rf.replicateTo(p, term) // will be empty and that’s okayyy
		}
	}()
}

func (rf *Raft) lastLogIndexLocked() int {
	return len(rf.log) - 1
}

func (rf *Raft) lastLogTermLocked() int {
	return rf.log[len(rf.log)-1].Term
}

func (rf *Raft) termAtLocked(i int) int {
	if i == 0 {
		return 0
	}
	if i < 0 || i >= len(rf.log) {
		return -1
	}
	return rf.log[i].Term
}
func (rf *Raft) logContainsLocked(prevIndex int, prevTerm int) bool {
	if prevIndex == 0 {
		return prevTerm == 0
	}
	if prevIndex < 0 || prevIndex >= len(rf.log) {
		return false // follower is too short or invalid index
	}
	return rf.log[prevIndex].Term == prevTerm
}

type Raft struct {
	mu        sync.Mutex
	peers     []*labrpc.ClientEnd
	persister *Persister
	me        int // index into peers[]

	state    state // leader, candidate, or follower
	term     int
	votedFor int //index of person i voted for in this term

	electionTimeout time.Duration // timer that triggers election (follower) or reelection (candidate)
	lastHeartbeat   time.Time
	voteCount       int // (candidate only) how many votes recieved in this term

	log         []LogEntry
	commitIndex int
	lastApplied int

	nextIndex  []int // (leader only)
	matchIndex []int // (leader only)

	applyCh chan ApplyMsg
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	term := rf.term
	isleader := false

	if rf.state == leader {
		isleader = true
	}
	//rf.say("im being asked if im a leader (%v) in term #%d", isleader, term)
	return term, isleader
}

// -------------------------------------- REQUESTVOTE  --------------------------------------
type RequestVoteArgs struct {
	ElectionTerm int
	CandidateID  int

	LastLogIndex int
	LastLogTerm  int
}
type RequestVoteReply struct {
	ElectionTerm int // used to determine if the candidate itself is in an older term
	Vote         bool
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// decide to vote or not
	rf.mu.Lock()
	defer rf.mu.Unlock()

	//rf.say("I'm being asked to vote")
	reply.ElectionTerm = rf.term
	reply.Vote = false

	if args.ElectionTerm < rf.term { //candidate is old
		//rf.say("I did NOT vote for {%d} because he was old {his term #%d vs my term #%d}", args.CandidateID, args.ElectionTerm, rf.term)
		return
	} else if args.ElectionTerm > rf.term {
		rf.stepDownLocked(args.ElectionTerm)
	} // same term from here down

	// up-to-date check
	myLastIdx := rf.lastLogIndexLocked()
	myLastTerm := rf.lastLogTermLocked()
	upToDate := args.LastLogTerm > myLastTerm ||
		(args.LastLogTerm == myLastTerm && args.LastLogIndex >= myLastIdx)

	if (rf.votedFor == -1 || rf.votedFor == args.CandidateID) && upToDate {
		rf.votedFor = args.CandidateID
		reply.Vote = true
		reply.ElectionTerm = rf.term
	}
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	// used by candidate
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// -------------------------------------- ELECTION  --------------------------------------

func (rf *Raft) StartElection() {

	rf.mu.Lock()
	rf.resetElectionTimeoutLocked("StartElection")

	// get info while locked
	rf.term++
	rf.state = candidate
	rf.votedFor = rf.me
	rf.voteCount = 1 // vote for self
	term := rf.term
	me := rf.me
	nPeers := len(rf.peers)
	lastIdx := rf.lastLogIndexLocked()
	lastTerm := rf.lastLogTermLocked()

	rf.updateLastHeartbeatLocked("StartElection")
	//rf.say("starting election for term %d (I have %d peers)", term, nPeers)

	// quick skip (esp when its just 1 node)
	if rf.voteCount > nPeers/2 {
		rf.becomeLeaderLocked()
		rf.mu.Unlock()
		return
	}
	rf.mu.Unlock()

	args := &RequestVoteArgs{
		ElectionTerm: term,
		CandidateID:  me,
		LastLogIndex: lastIdx,
		LastLogTerm:  lastTerm,
	}
	for peer := range rf.peers { // for every peer
		if peer == me {
			continue
		}
		go func(peerID int) {
			// need to be unlocked for sendRequestVote

			reply := RequestVoteReply{}

			ok := rf.sendRequestVote(peerID, args, &reply)

			if !ok {
				//rf.say("my call failed")
				return
			}

			rf.mu.Lock()

			if reply.ElectionTerm > rf.term {
				rf.stepDownLocked(reply.ElectionTerm)
				rf.mu.Unlock()
				return
			}

			if reply.Vote {
				rf.voteCount++
				if rf.voteCount > nPeers/2 && rf.state == candidate && rf.term == term {
					rf.becomeLeaderLocked()
					rf.mu.Unlock()
					rf.StartHeartbeat()
					return
				}
			}
			rf.mu.Unlock()
		}(peer)

	}

}

// -------------------------------------- HEARTBEAT  --------------------------------------
type HeartbeatArgs struct {
	LeaderID int
	Term     int
	Entry    int // empty if actual heartbeat
}

type HeartbeatReply struct {
	Term   int
	Status bool
}

func (rf *Raft) Heartbeat(args *HeartbeatArgs, reply *HeartbeatReply) {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.term
	reply.Status = false

	rf.say("heartbeat recieved from {%d} in term #%d", args.LeaderID, args.Term)
	//rf.say("my state is {%s}", rf.state)
	if args.Term < rf.term {
		//rf.say("I'm ignoring an old heartbeat from {%d}", args.LeaderID)
		return
	}
	if args.Term > rf.term {
		rf.term = args.Term
		rf.votedFor = -1
	}
	if rf.state != follower {
		rf.state = follower
	}

	rf.updateLastHeartbeatLocked("Heartbeat")

	reply.Term = rf.term
	reply.Status = true
}

func (rf *Raft) sendHeartbeat(server int, args *HeartbeatArgs, reply *HeartbeatReply) bool {
	//used by leader
	ok := rf.peers[server].Call("Raft.Heartbeat", args, reply)
	return ok
}

func (rf *Raft) StartHeartbeat() {
	rf.mu.Lock()

	rf.updateLastHeartbeatLocked("StartHeartbeat")
	// get info while locked
	term := rf.term
	me := rf.me
	n := len(rf.peers)

	rf.mu.Unlock()

	for peer := 0; peer < n; peer++ {
		if peer == me {
			continue
		}
		go func(peerID int, t int) {
			args := HeartbeatArgs{LeaderID: me, Term: t}
			var reply HeartbeatReply
			rf.say("sending heartbeat to {%d} for term #%d", peerID, t)
			rf.sendHeartbeat(peerID, &args, &reply)
			if reply.Term > t {
				rf.mu.Lock()
				rf.stepDownLocked(reply.Term)
				rf.mu.Unlock()
			}
			rf.replicateTo(peerID, t)
		}(peer, term)
	}
}

// -------------------------------------- LIFE --------------------------------------

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.say("I've been created :D")
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	rf.state = follower
	rf.term = 0
	rf.votedFor = -1
	rf.electionTimeout = 0
	rf.voteCount = 0

	rf.log = []LogEntry{{Term: 0}} // temp 0
	rf.commitIndex = 0
	rf.lastApplied = 0

	// randomize
	rf.resetElectionTimeoutLocked("Make")
	rf.updateLastHeartbeatLocked("Make")

	rf.applyCh = applyCh
	go rf.applier()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	go rf.life()

	return rf
}

// this will control all the logic
func (rf *Raft) life() {
	for {

		rf.mu.Lock()

		// for some reason timers are so weird to work with
		// i believe some of my inconsistencies are because of it
		// so im gonna use timestamps

		state := rf.state
		term := rf.term
		lastHeartbeat := rf.lastHeartbeat
		timeout := rf.electionTimeout

		rf.mu.Unlock()

		//no locks
		if state == follower {
			rf.say("I'm a follower in term #%d", term)
			time.Sleep(100 * time.Millisecond)
			if time.Since(lastHeartbeat) > timeout {
				rf.StartElection()
			}
			time.Sleep(100 * time.Millisecond)

		} else if state == candidate {
			rf.say("I'm a candidate in term #%d", term)
			if time.Since(lastHeartbeat) > timeout {
				rf.say("gonna try re election")
				rf.StartElection()
			}
			time.Sleep(100 * time.Millisecond)

		} else if state == leader {
			rf.say("I'm a LEADER in term #%d", term)
			rf.StartHeartbeat()
			time.Sleep(500 * time.Millisecond)
		}

	}

}

// -------------------------------------- APPENDENTRIES END --------------------------------------

type AppendEntriesArgs struct {
	Term         int
	LeaderID     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool

	// for correcting
	ConflictTerm  int
	ConflictIndex int
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	rf.say("recieved AppendEntries")

	reply.Term = rf.term
	reply.Success = false
	reply.ConflictTerm = 0
	reply.ConflictIndex = 0

	if args.Term < rf.term {
		rf.say("I'm ignoring old AppendEntry")
		return
	}
	if args.Term > rf.term {
		rf.stepDownLocked(args.Term)
	} else if rf.state != follower {
		rf.state = follower
	}
	rf.updateLastHeartbeatLocked("AppendEntries")

	// consistency check
	if args.PrevLogIndex > rf.lastLogIndexLocked() {
		// follower too short
		reply.ConflictTerm = -1
		reply.ConflictIndex = rf.lastLogIndexLocked() + 1
		return
	}
	if args.PrevLogIndex >= 0 && rf.termAtLocked(args.PrevLogIndex) != args.PrevLogTerm { // term mismatch
		rf.say("found log term mismatch")
		// find conflict term and its first index
		ct := rf.termAtLocked(args.PrevLogIndex)
		reply.ConflictTerm = ct
		// find first index of ct
		i := args.PrevLogIndex
		for i > 0 && rf.termAtLocked(i-1) == ct {
			i--
		}
		reply.ConflictIndex = i
		return
	}

	// merge entries
	rf.syncLocked(args.PrevLogIndex+1, args.Entries)
	reply.Success = true
	reply.Term = rf.term

	// commitIndex update
	if args.LeaderCommit > rf.commitIndex {
		max := rf.lastLogIndexLocked()
		if args.LeaderCommit < max {
			max = args.LeaderCommit
		}
		rf.commitIndex = max
	}
	rf.say("appended to log successfully yayayay")
	rf.say("heres my log %v", rf.log)
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	//used by leader
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) syncLocked(startIndex int, entries []LogEntry) {
	if len(entries) == 0 {
		return
	}

	// making sure the index is valid
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(rf.log) {
		startIndex = len(rf.log) // clamp buwop
	}

	// skip any matching entries to avoid unnecessary truncation
	i, j := startIndex, 0
	for i < len(rf.log) && j < len(entries) && rf.log[i].Term == entries[j].Term {
		i++
		j++
	}

	// T[:i:i] prevents reuse of the old tail’s capacity
	rf.log = rf.log[:i:i]

	// append only the mismatched suffix from leader
	if j < len(entries) {
		rf.log = append(rf.log, entries[j:]...)
	}
}

func (rf *Raft) replicateTo(peer int, leaderTerm int) {
	for {
		rf.mu.Lock()

		// abort if we’re no longer the right leader/term
		if rf.state != leader || rf.term != leaderTerm {
			rf.mu.Unlock()
			return
		}

		next := rf.nextIndex[peer]
		if next < 1 {
			next = 1
			rf.nextIndex[peer] = 1
		}
		prevIdx := next - 1
		prevTerm := rf.termAtLocked(prevIdx)
		entries := append([]LogEntry(nil), rf.log[next:]...)

		args := AppendEntriesArgs{
			Term:         rf.term,
			LeaderID:     rf.me,
			PrevLogIndex: prevIdx,
			PrevLogTerm:  prevTerm,
			Entries:      entries,        // can be empty
			LeaderCommit: rf.commitIndex, // to propagate commit
		}
		rf.mu.Unlock()

		var reply AppendEntriesReply
		if !rf.peers[peer].Call("Raft.AppendEntries", &args, &reply) {
			return
		}

		rf.mu.Lock()

		if reply.Term > rf.term { // im old
			rf.stepDownLocked(reply.Term)
			rf.mu.Unlock()
			return
		}

		if rf.term != leaderTerm || rf.state != leader {
			rf.mu.Unlock()
			return
		}

		if reply.Success {
			// follower caught up through args.PrevLogIndex + len(entries)
			match := args.PrevLogIndex + len(entries)
			if match > rf.matchIndex[peer] {
				rf.matchIndex[peer] = match
				rf.nextIndex[peer] = match + 1
			}
			// try to advance commit for this leader term
			rf.tryAdvanceCommitLocked()
			rf.mu.Unlock()
			return // done with this peer for now mwahaha
		}

		// fast backtrack
		if reply.ConflictTerm == -1 { // follower too short
			rf.nextIndex[peer] = reply.ConflictIndex
		} else {
			// find last index of ConflictTerm on leader
			last := -1
			for i := rf.lastLogIndexLocked(); i >= 1; i-- {
				if rf.log[i].Term == reply.ConflictTerm {
					last = i
					break
				}
				if rf.log[i].Term < reply.ConflictTerm {
					break
				}
			}
			if last != -1 {
				rf.nextIndex[peer] = last + 1
			} else {
				rf.nextIndex[peer] = reply.ConflictIndex
			}
		}
		if rf.nextIndex[peer] < 1 {
			rf.nextIndex[peer] = 1
		}

		// do it again with the updated nextIndex
		rf.mu.Unlock()
	}
}

func (rf *Raft) tryAdvanceCommitLocked() {
	for N := rf.lastLogIndexLocked(); N > rf.commitIndex; N-- {
		if rf.log[N].Term != rf.term {
			continue
		}
		count := 1 // including leader
		for i := range rf.peers {
			if i != rf.me && rf.matchIndex[i] >= N {
				count++
			}
		}
		if count > len(rf.peers)/2 {
			rf.commitIndex = N
			break
		}
	}
}
func (rf *Raft) applier() {
	for {
		rf.mu.Lock()
		for rf.lastApplied < rf.commitIndex {
			rf.lastApplied++
			idx := rf.lastApplied
			cmd := rf.log[idx].Command
			msg := ApplyMsg{Index: idx, Command: cmd}
			rf.mu.Unlock()
			rf.applyCh <- msg
			rf.mu.Lock()
		}
		rf.mu.Unlock()
		time.Sleep(2 * time.Millisecond) // be careful of the sleep pls
	}
}

func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()

	if rf.state != leader {
		term := rf.term
		rf.mu.Unlock()
		return -1, term, false
	}

	// append locally
	entry := LogEntry{Term: rf.term, Command: command}
	rf.log = append(rf.log, entry)
	index := len(rf.log) - 1
	term := rf.term

	// leader's replication state
	rf.matchIndex[rf.me] = index
	rf.nextIndex[rf.me] = index + 1

	nPeers := len(rf.peers)
	rf.mu.Unlock()

	rf.say("I appended locally")

	// nudge followers immediately
	for i := 0; i < nPeers; i++ {
		if i == rf.me {
			continue
		}
		go rf.replicateTo(i, term)
	}

	rf.say("I asked all of my peers to append")

	return index, term, true
}

func (rf *Raft) Kill() {
	// Your code here, if desired.

	// thank you i dont desire anything
}

func (rf *Raft) persist() {
	// Your code here.
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
}
