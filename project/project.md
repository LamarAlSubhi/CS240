# Project 
Prof suggested changes:

- run gossip protocols across separate processes rather than within a single simulator. 
- deploy the nodes inside a Mininet environment and experiment with controllable delay, bandwidth, and churn.


## Workflow? / plan? / something?

1. Message format + basic push on reliable links
2. Push-pull + TTL
3. Anti-entropy reconciliation
4. Mininet bring-up
5. Baseline experiments (convergence vs N, f) in Mininet
6. Loss, jitter, bandwidth limits, and churn
7. Analysis & plots
8. Report & slides


## HOW TO RUN (hassan pls look):

### TERMINAL 1:
- cd CS240/project/gossip
- go build -o gossipd ./cmd/gossipd

### TERMINAL 2:
- ./gossipd --id=1 --bind=:9001 --seeds=127.0.0.1:9002 --metrics-addr=:9081 --log=/tmp/g1.jsonl

### TERMINAL 3:
- ./gossipd --id=2 --bind=:9002 --seeds=127.0.0.1:9001 --metrics-addr=:9082 --log=/tmp/g2.jsonl

### TERMINAL 1 (again):
- curl "http://localhost:9081/inject?id=r3&body=kaust&ttl=6"
