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

import "sync"
import "labrpc"

import "bytes"
import "encoding/gob"
import "fmt"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

// Entry structure in log
type LogEntry struct {
    Index   int
    Command interface{}
	Term    int
}


//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex
	peers     []*labrpc.ClientEnd
	persister *Persister
	me        int // index into peers[]

	// Your data here.
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	//state
	State string

	// Persist on all servers
	CurrentTerm   int
	VotedFor      int
	Log           []LogEntry

	//Volatile state on all servers
	CommitIndex   int
	LastApplied   int

	// Volatile state on leaders
	NextIndex     []int
	MatchIndex    []int

	//chan for communication
    HeartBeatCH   chan *AppendEntriesArgs
	ApplyCH       chan ApplyMsg
	QuitCH        chan bool
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here.
	term = rf.CurrentTerm
	isleader = (rf.State == "Leader")
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here.
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)

	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)
	e.Encode(rf.CurrentTerm)
	e.Encode(rf.VotedFor)
	e.Encode(rf.Log)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)
	d.Decode(&rf.CurrentTerm)
	d.Decode(&rf.VotedFor)
    d.Decode(&rf.Log)
}

//
// example RequestVote RPC arguments structure.
//
type RequestVoteArgs struct {
	// Your data here.
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

//
// example RequestVote RPC reply structure.
//
type RequestVoteReply struct {
	// Your data here.
	Term        int
	VoteGranted bool
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
    ConflictEntry int
    Term          int
	Success       bool
}

//
// example RequestVote RPC handler.
// 1. Reply false if term < currentTerm.
// 2. If votedFor is null or candidateId, and candidate’s log is at least as up-to-date as receiver’s log, grant vote.
//
func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here.
    rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()

    //1. Reply false if term < currentTerm.
    if args.Term < rf.CurrentTerm {
    	//reply.Term =
        reply.VoteGranted = false
        fmt.Println("%v denies the vote from %v because stale\n", rf.me, args.CandidateId)
        return
    }

    //if receiving higher term, update votedFor, CurrentTerm, and transfer to follower
    if rf.CurrentTerm < args.Term {
		rf.CurrentTerm = args.Term
		rf.VotedFor = -1
		rf.State = "Follower"
	}

    //2. If votedFor is null or candidateId, and candidate’s log is at least as up-to-date as receiver’s log, grant vote.
    reply.Term = rf.CurrentTerm
    receiverLogIndex := rf.Log[len(rf.Log) - 1].Index
	receiverLogTerm := rf.Log[len(rf.Log) - 1].Term
	if (rf.VotedFor == -1 || rf.VotedFor == args.CandidateId) && ((receiverLogTerm < args.LastLogTerm) || (receiverLogTerm == args.LastLogTerm && receiverLogIndex <= args.LastLogIndex)) {
        rf.VotedFor = args.CandidateId
        reply.VoteGranted = true
        fmt.Println("%v term %v vote for %v term %v\n", rf.me, rf.CurrentTerm, args.CandidateId, args.Term)
	}
	else
	{
        reply.VoteGranted = false
        fmt.Println("%v denies the vote from %v because has voted or log is invalid\n", rf.me, args.CandidateId)
	}
	return
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// returns true if labrpc says the RPC was delivered.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}


//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true


	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
    rf.QuitCH<-true
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.mu.Lock()
	defer rf.mu.Unlock()

	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here.
    rf.CurrentTerm = 0
    rf.VotedFor = -1
    rf.Log = make([]LogEntry, 1)
	rf.Log[0] = LogEntry{0, 0, 0}

    rf.ApplyCH = applyCh
    rf.HeartBeatCH = make(chan *AppendEntriesArgs, 1)
    //rf.rand = rand.New(rand.NewSource(int64(rf.me)))

    rf.State = "Follower"

    rf.CommitIndex = 0
	rf.LastApplied = 0

	rf.QuitCH = make(chan bool, 1)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	rf.LastApplied = rf.Log[0].Index
	rf.CommitIndex = rf.Log[0].Index

	fmt.Println("new server %v is up, log size %v, commitId %v\n", me, len(rf.Log), rf.CommitIndex)

    //begin from follower, ready to receive heartbeat
	go func() {
		rf.FollowerState()
	}()

	return rf
}

func (rf *Raft) FollowerState() {
	fmt.Println(rf.me, " enter into FollowerState Function")

	electiontimeout := 100
	randomized := electiontimeout + rand.Intn(2*electiontimeout)
	nexttimeout := time.Duration(randomized)
	t := time.NewTimer(nexttimeout * time.Millisecond)
	for {
        if rf.State != "Follower" {
    	    fmt.Println(rf.me, " enter into FollowerState, but not Follower")
		    return
	    }

        // loop until time out or receive a correct heartbeat
		endLoop := false
        for !endLoop {
            select {
		    case <- t.C:
		    	// time out, end the heartbeat timer and fire a new election Term
		    	fmt.Println(rf.me, " became candidate")
			    go rf.CandidateState(rf.CurrentTerm + 1)
			    return
		    case msg := <-rf.HeartBeatCH:
		    	if rf.CurrentTerm > msg.Term {
					// stale heart beat, ignore and continue the loop
					fmt.Println(rf.me, " receive a stale heartbeat")
				}
				else {
					// receive a legal heartbeat, break the loop to wait next heartBeat
					rf.mu.Lock()
					rf.CurrentTerm = msg.Term
					rf.VotedFor = msg.LeaderId
					rf.persist()
					rf.mu.Unlock()
					randomized  = electiontimeout + rand.Intn(2*electiontimeout)
			        nexttimeout = time.Duration(randomized)
			        t.Reset(nexttimeout * time.Millisecond)
					endLoop = true
				}
		    case <- rf.QuitCH:
			    fmt.Println(rf.me, " receive kill signal")
			    return
		    }
        }
	}
}

func (rf *Raft) CandidateState(electionTerm int) {
	fmt.Println(rf.me, " enter into CandidateState Function")

    // increase currentTerm, and vote for self
	rf.mu.Lock()
	rf.State = "Candidate"
    rf.CurrentTerm = electionTerm
	rf.VotedFor = rf.me
    rf.mu.Unlock()

    fmt.Println("new election begin in %v, Term %v\n", rf.me, rf.CurrentTerm)

	//construct request vote message
	args := RequestVoteArgs{rf.CurrentTerm, rf.me, rf.Log[len(rf.Log)-1].Index, rf.Log[len(rf.Log)-1].Term}

    type Rec struct {
		ok bool
		reply *RequestVoteReply
	}
	recBuff := make(chan Rec, 1)
	for i := range rf.peers {
		if i != rf.me {
			// send requestVote in parallel
			go func(server int) {
				reply := RequestVoteReply{0, false}
			    ok := rf.sendRequestVote(server, args, reply)
			    fmt.Println("in election %v get reply %v\n", rf.me, reply)
			    recBuff <- Rec {ok, reply}
			}(i)
		}
	}

	// signal: wins the election
	winSignal := make(chan bool, 1)
	// signal: my current Term is out of date
	staleSignal := make(chan *RequestVoteReply, 1)
	failSignal := make(chan bool)
	go func(){
		// get an approve from myself
		approveNum := 1
		denyNum := 0
		for i := 0; i < len(rf.peers) - 1; i++{
			rec := <- recBuff
			if !rec.ok {
				continue
			}
			if rec.reply.VoteGranted{
				approveNum++
				if approveNum > len(rf.peers) / 2{
					winSignal <- true
					break
				}
			}else{
				if rec.reply.Term > rf.CurrentTerm {
					staleSignal <- rec.reply
					break
				}

				denyNum++
				if denyNum > len(rf.peers) / 2 {
					failSignal <- true
					break
				}

			}
		}
	}()

	electiontimeout := 100
	randomized  := electiontimeout + rand.Intn(2*electiontimeout)
	nexttimeout := time.Duration(randomized)
	t := time.NewTimer(nexttimeout * time.Millisecond)

	for {
		select {
		case <- t.C:
			rf.becomesCandidate()
			return
		case <-winSignal:
			rf.mu.Lock()
			rf.State = "Leader"
			rf.NextIndex = make([]int, len(rf.peers))
	        rf.MatchIndex = make([]int, len(rf.peers))
			for i := 0; i < len(rf.peers); i++ {
				rf.NextIndex[i] = rf.Log[len(rf.Log)-1].Index+1
		        rf.MatchIndex[i] = rf.Log[0].Index
			}
			rf.mu.Unlock()
			fmt.Println("candidate %v becomes leader in Term %v, log size %v\n", rf.me, rf.CurrentTerm, len(rf.Log))
			go rf.broadcast()
			return
		case <- failSignal:
			rf.mu.Lock()
			rf.State = "Follower"
			rf.VotedFor = -1
			rf.mu.Unlock()
			rf.persist()
			go rf.FollowerState()
			return
		case reply := <-staleSignal:
			// discover a new Term
		    // turn into follower state
			rf.mu.Lock()
			rf.CurrentTerm = reply.Term
			rf.State = "Follower"
			rf.VotedFor = -1
			rf.mu.Unlock()
			rf.persist()
			go rf.FollowerState()
			return
		case msg := <- rf.HeartBeatCH:
            if msg.Term < rf.CurrentTerm {
				// receive stale heartbeat, ignore
				break
			}
			// fail the election, get heartbeat from other leader
			rf.mu.Lock()
			rf.CurrentTerm= msg.Term
			rf.State = "Follower"
			rf.VotedFor = msg.LeaderId
			rf.mu.Unlock()
			go rf.FollowerState()
			fmt.Println("candidate %v becomes follower\n", rf.me)
			return
		case <- rf.QuitCH:
			return
		}
	}
}














