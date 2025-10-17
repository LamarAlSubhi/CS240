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
	minElectionTimeout       = 500 * time.Millisecond
	maxElectionTimeout       = 1000 * time.Millisecond
	heartbeatInterval        = 100 * time.Millisecond
)

func (rf *Raft) say(format string, a ...interface{}) {
	prefix := fmt.Sprintf("[%d]: ", rf.me)
	fmt.Printf(prefix+format+"\n", a...)
}

func (rf *Raft) resetElectionTimerLocked(by string) {
	// pick a random duration between 150ms and 300ms
	delta := rand.Int63n(int64(maxElectionTimeout - minElectionTimeout))
	timeout := minElectionTimeout + time.Duration(delta)
	// boom randomized each time
	if rf.electionTimer != nil {
		rf.electionTimer.Stop()
	}
	rf.electionTimer = time.NewTimer(timeout)
	rf.say("just reset my election timer by (%s)", by)
}

func (rf *Raft) resetHeartbeatTickerLocked() {
	if rf.heartbeatTicker != nil {
		rf.heartbeatTicker.Stop()
	}
	rf.heartbeatTicker = time.NewTicker(heartbeatInterval)
	rf.say("just reset my heartbeat ticker")
}

func (rf *Raft) stepDownLocked(term int, leader int) {
	rf.votedFor = leader
	rf.term = term
	rf.state = follower
	rf.say("I stepped down for {%d} my new term is #%d", rf.votedFor, rf.term)
}
func (rf *Raft) becomeCandidateLocked() {
	rf.votedFor = rf.me
	rf.voteCount = 1
	rf.term++
	rf.state = candidate
	rf.say("I'm now a candidate in term #%d", rf.term)
}
func (rf *Raft) becomeLeaderLocked() {
	rf.say("I am now a leader in term #%d (won with %d votes)", rf.term, rf.voteCount)
	rf.voteCount = 0
	rf.state = leader
	rf.StartHeartbeat()
	rf.say("sent heartbeats")
	rf.resetElectionTimerLocked("becomeLeaderLocked")
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

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
	rf.mu.Lock()

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
	args := RequestVoteArgs{
		ElectionTerm: rf.term,
		CandidateID:  rf.me,
	}

	rf.resetElectionTimerLocked("StartElection")

	rf.say("i have #%d peers", len(rf.peers))

	for peer := range rf.peers { // for every peer

		go func(peerID int) {
			// need to be unlocked for sendRequestVote

			if peerID != rf.me {

				reply := RequestVoteReply{}

				ok := rf.sendRequestVote(peerID, args, &reply)

				if ok {
					if args.ElectionTerm == rf.term {
						if reply.Vote {
							rf.voteCount++
							rf.say("recieved vote -> current vote count is %d", rf.voteCount)

						} else {
							rf.say("no vote -> current vote count is %d", rf.voteCount)
						}
					} else if reply.ElectionTerm > rf.term { // im alerted that im old
						rf.stepDownLocked(reply.ElectionTerm, -1)
					}
				} else {
					rf.say("my call failed")
				}
			}
			if rf.voteCount > (len(rf.peers) / 2) { // majority reached
				rf.becomeLeaderLocked()
			}
		}(peer)

	}
	rf.mu.Unlock()
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

	rf.say("heartbeat recieved from {%d}", args.LeaderID)
	if args.Term < rf.term {
		rf.say("I'm ignoring an old heartbeat from {%d}", args.LeaderID)
	}
	if rf.state == candidate || rf.state == leader {
		rf.stepDownLocked(args.Term, args.LeaderID)
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
				rf.say("sending heartbeat to {%d}", peerID)
				rf.sendHeartbeat(peerID, &args, &reply)
			}(peer)
		}
	}
}

// -------------------------------------- HEARTBEAT END --------------------------------------

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

	// randomize
	rf.resetElectionTimerLocked("Make")

	rf.say("I've been created :D")

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	go rf.life()

	return rf
}

// this will control all the logic
func (rf *Raft) life() {
	for {

		rf.mu.Lock()

		state := rf.state
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

			if state != leader {
				rf.mu.Lock()
				rf.resetElectionTimerLocked("life loop")
				rf.becomeCandidateLocked()
				rf.mu.Unlock()
				rf.StartElection()
			}

		case <-heartbeatCh: //timeout
			rf.say("need to resend hearbeats (heartbeat timeout)")
			rf.mu.Lock()
			rf.StartHeartbeat()
			rf.mu.Unlock()
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
