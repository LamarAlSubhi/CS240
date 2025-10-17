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
	"sync"
	"time"
)

// import "bytes"
// import "encoding/gob"

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

// -------------------------------------- HELPERS START --------------------------------------

// defining the states a node can be
type state string

const (
	leader             state = "leader"
	candidate          state = "candidate"
	follower           state = "follower"
	minElectionTimeout       = 150 * time.Millisecond
	maxElectionTimeout       = 300 * time.Millisecond
	heartbeatInterval        = 100 * time.Millisecond
)

func (rf *Raft) say(format string, a ...interface{}) {
	prefix := fmt.Sprintf("[%d]: ", rf.me)
	fmt.Printf(prefix+format+"\n", a...)
}

func (rf *Raft) resetElectionTimer() {
	// pick a random duration between 150ms and 300ms
	delta := rand.Int63n(int64(maxElectionTimeout - minElectionTimeout))
	timeout := minElectionTimeout + time.Duration(delta)
	// boom randomized each time
	if rf.electionTimer != nil {
		rf.electionTimer.Stop()
	}
	rf.electionTimer = time.NewTimer(timeout)
	rf.say("just reset my election timer")
}

func (rf *Raft) resetHeartbeatTicker() {
	if rf.heartbeatTicker != nil {
		rf.heartbeatTicker.Stop()
	}
	rf.heartbeatTicker = time.NewTicker(heartbeatInterval)
	rf.say("just reset my heartbeat ticker")
}

// -------------------------------------- HELPERS END --------------------------------------
type Raft struct {
	mu        sync.Mutex
	peers     []*labrpc.ClientEnd
	persister *Persister
	me        int // index into peers[]

	state    state // leader, candidate, or follower
	term     int
	votedFor int //index of person i voted for in this term

	electionTimer *time.Timer // timer that triggers election (follower) or reelection (candidate)
	voteCount     int         // (candidate only) how many votes recieved in this term

	heartbeatTicker *time.Ticker // (leader only) timer that leader uses to send heartbeats
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	term := rf.term
	isleader := false

	if rf.state == leader {
		isleader = true
	}

	return term, isleader
}

// -------------------------------------- REQUESTVOTE START --------------------------------------
type RequestVoteArgs struct {
	ElectionTerm int
	CandidateID  int
}
type RequestVoteReply struct {
	ElectionTerm int // used to determine if the candidate itself is in an older term
	Vote         bool
}

func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	// decide to vote or not

	rf.say("I'm being asked to vote")
	reply.ElectionTerm = rf.term
	reply.Vote = false

	if args.ElectionTerm < rf.term { //candidate is old
		rf.say("I did NOT vote for {%d} because he was old {his term #%d vs my term #%d}", args.CandidateID, args.ElectionTerm, rf.term)
	} else if args.ElectionTerm > rf.term { // im older, i should vote
		// update my info
		rf.votedFor = args.CandidateID
		rf.term = args.ElectionTerm

		reply.ElectionTerm = rf.term
		reply.Vote = true

		rf.say("I just voted for {%d} in term #%d", rf.votedFor)
	} else { // we're on equal terms hahahahahaha GET IT???
		if rf.votedFor == -1 { // didnt vote yet

			rf.votedFor = args.CandidateID
			rf.state = follower // reinforce in case I was also candidate
			reply.Vote = true
			reply.ElectionTerm = rf.term

		} else {
			// i couldve voted for myself or for someone else
			rf.say("I already voted for {%d} in term #%d {sorry mr. %d }", rf.votedFor, rf.term, args.CandidateID)
		}
	}
	rf.resetElectionTimer()
}

func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	// used by candidate
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// -------------------------------------- REQUESTVOTE END --------------------------------------
// -------------------------------------- ELECTION START --------------------------------------

// -------------------------------------- ELECTION END --------------------------------------

// -------------------------------------- LIFE START --------------------------------------

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here.
	rf.state = follower
	rf.term = 0
	rf.votedFor = -1
	rf.electionTimer = time.NewTimer(maxElectionTimeout)
	rf.voteCount = 0
	rf.heartbeatTicker = time.NewTicker(heartbeatInterval)

	// randomize
	rf.resetElectionTimer()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}

// this will control all the logic
func (rf *Raft) life() {
	// might have lock issues

	if rf.state == follower {

	} else if rf.state == candidate {

	} else if rf.state == leader {
	}

}

// -------------------------------------- LIFE END --------------------------------------

func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	return index, term, isLeader
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
