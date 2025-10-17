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
1. define roles
2. implement the timeouts
3. args and reply for RPC's for heartbeats and votes
4. empty AppendEntries as heartbeat
5. election logic
6. logs
7. idk yet

