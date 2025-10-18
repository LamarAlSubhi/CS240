## as3: Raft

### timeouts:
- election timeout [follower]: 
    - trigger: no heartbeat recieved
    - at timeout: follower becomes candidate
- re-election timeout [candidate]: 
    - trigger: no majority votes achieved
    - at timeout: try again with newer term

### intervals:
- heartbeat interval [leader]:
    - send empty AppendEntries

### to do list:
1. define states
2. implement the timeouts
3. args and reply for RPC's for heartbeats and votes
4. empty AppendEntries as heartbeat
5. election logic
6. logs
7. idk yet

### rpcs needed
i need a way to communicate the following:
- RequestVote: when candidate is asking for votes (to peers)
- VoteRequested: when peers vote (to candidate)

- AppendEntries: when leaders send an append order (to peers)
- EntriesAppended: when peers append an entry (to leader)

- AppendEntries: when leaders send a heartbeat (to peers)
- EntriesAppended: when peers recieve a heartbeat (to leader)

- when leaders win an election (to followers),
- when leaders apply a message (to client),


### logs
