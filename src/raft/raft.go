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
	minElectionTimeout       = 800 * time.Millisecond
	maxElectionTimeout       = 1600 * time.Millisecond
	heartbeatInterval        = 100 * time.Millisecond
)

func (rf *Raft) say(format string, a ...interface{}) {
	prefix := fmt.Sprintf("[%d]: ", rf.me)
	fmt.Printf(prefix+format+"\n", a...)
}

func (rf *Raft) resetElectionTimerLocked(by string) {
	d := time.Duration(rand.Int63n(int64(maxElectionTimeout-minElectionTimeout))) + minElectionTimeout
	if rf.electionTimer == nil {
		rf.electionTimer = time.NewTimer(d)
		rf.say("created election timer (%s)", by)
		return
	}
	if !rf.electionTimer.Stop() {
		select {
		case <-rf.electionTimer.C:
		default:
		}
	}
	rf.electionTimer.Reset(d)
	rf.say("just reset my election timer by (%s)", by)
}

func (rf *Raft) stepDownLocked(term int) {

	if term > rf.term {
		rf.term = term
	}
	if rf.heartbeatTicker != nil {
		rf.heartbeatTicker.Stop()
		rf.heartbeatTicker = nil
	}
	rf.votedFor = -1
	rf.state = follower
	rf.voteCount = 0
	rf.resetElectionTimerLocked("stepDown")
	rf.say("stepped down to follower, term=%d", rf.term)
}
func (rf *Raft) becomeCandidateLocked() {
	rf.votedFor = rf.me
	rf.voteCount = 1
	rf.term++
	rf.state = candidate
	rf.say("I'm now a candidate in term #%d", rf.term)
}
func (rf *Raft) becomeLeaderLocked() {
	rf.say("I am now a leader in term #%d (won with %d votes) YAY !!!!!!!!!!!!!!!!", rf.term, rf.voteCount)
	rf.state = leader

	if rf.electionTimer != nil {
		if !rf.electionTimer.Stop() {
			select {
			case <-rf.electionTimer.C:
			default:
			}
		}
	}
	if rf.heartbeatTicker != nil {
		rf.heartbeatTicker.Stop()
	}
	rf.heartbeatTicker = time.NewTicker(heartbeatInterval)
}

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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	term := rf.term
	isleader := false

	if rf.state == leader {
		isleader = true
	}
	rf.say("im being asked if im a leader (%v) in term #%d", isleader, term)
	return term, isleader
}

// -------------------------------------- REQUESTVOTE  --------------------------------------
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
	rf.mu.Lock()

	rf.say("I'm being asked to vote")
	reply.ElectionTerm = rf.term
	reply.Vote = false

	if args.ElectionTerm < rf.term { //candidate is old
		rf.say("I did NOT vote for {%d} because he was old {his term #%d vs my term #%d}", args.CandidateID, args.ElectionTerm, rf.term)
	} else if args.ElectionTerm > rf.term { // im older, i should vote
		rf.stepDownLocked(args.ElectionTerm)
		rf.votedFor = args.CandidateID
		rf.state = follower
		reply.Vote = true
		reply.ElectionTerm = rf.term

		rf.say("I just voted for {%d} in term #%d", rf.votedFor, rf.term)
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
	rf.resetElectionTimerLocked("RequestVote")
	rf.mu.Unlock()
}

func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	// used by candidate
	ok := rf.peers[server].Call("Raft.RequestVote", args, &reply)
	return ok
}

// -------------------------------------- ELECTION  --------------------------------------

func (rf *Raft) StartElection() {

	rf.mu.Lock()

	rf.term++
	rf.state = candidate
	rf.votedFor = rf.me
	rf.voteCount = 1 // vote for self
	term := rf.term
	me := rf.me
	nPeers := len(rf.peers)

	rf.resetElectionTimerLocked("StartElection")
	rf.say("starting election for term %d (I have %d peers)", term, nPeers)

	// quick skip
	if rf.voteCount > nPeers/2 {
		rf.becomeLeaderLocked()
		rf.mu.Unlock()
		return
	}
	rf.mu.Unlock()

	args := RequestVoteArgs{ElectionTerm: term, CandidateID: me}
	for peer := range rf.peers { // for every peer
		if peer == me {
			continue
		}
		go func(peerID int) {
			// need to be unlocked for sendRequestVote

			reply := RequestVoteReply{}

			ok := rf.sendRequestVote(peerID, args, &reply)

			if !ok {
				rf.say("my call failed")
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

	rf.say("heartbeat recieved from {%d} in term #%d", args.LeaderID, args.Term)
	if args.Term < rf.term {
		rf.say("I'm ignoring an old heartbeat from {%d}", args.LeaderID)
	} else {
		rf.resetElectionTimerLocked("Hearbeat")
	}

	if rf.state == candidate || rf.state == leader {
		rf.stepDownLocked(args.Term)
	}
	// not using this yet?
	// reply.Term = rf.term
	// reply.Status =
	rf.mu.Unlock()
}

func (rf *Raft) sendHeartbeat(server int, args *HeartbeatArgs, reply *HeartbeatReply) bool {
	//used by leader
	ok := rf.peers[server].Call("Raft.Heartbeat", args, reply)
	return ok
}

func (rf *Raft) StartHeartbeat() {
	for peer := range rf.peers { // for every peer
		if peer != rf.me { // skip myself
			go func(peerID int) {
				args := HeartbeatArgs{rf.me, rf.term, 77} // random int for now
				reply := HeartbeatReply{}
				rf.say("sending heartbeat to {%d} for term #%d", peerID, rf.term)
				rf.sendHeartbeat(peerID, &args, &reply)
			}(peer)
		}
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

	// Your initialization code here.
	rf.state = follower
	rf.term = 0
	rf.votedFor = -1
	rf.electionTimer = nil
	rf.voteCount = 0

	// randomize
	rf.resetElectionTimerLocked("Make")

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	go rf.life()

	return rf
}

// this will control all the logic
func (rf *Raft) life() {
	for {

		rf.mu.Lock()

		var electionCh <-chan time.Time
		var heartbeatCh <-chan time.Time

		// only use election timer when not leader
		if rf.state != leader && rf.electionTimer != nil {
			electionCh = rf.electionTimer.C
		}

		// only use heartbeat ticker when leader
		if rf.state == leader && rf.heartbeatTicker != nil {
			heartbeatCh = rf.heartbeatTicker.C
		}
		rf.mu.Unlock()

		// no locks while waiting for timeouts
		select {

		case <-electionCh: // timeout
			rf.say("election timeout")

			rf.StartElection()

		case <-heartbeatCh: //timeout
			rf.say("need to resend hearbeats (heartbeat timeout)")
			rf.StartHeartbeat()
		}
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
