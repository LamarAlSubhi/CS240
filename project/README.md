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


## HOW TO RUN

### STEP1: build the linux binary:
- cd project/gossip
- GOOS=linux GOARCH=amd64 go build -o ../bin/linux/gossipd ./cmd/gossipd

### STEP2: run Docker:

### STEP3: run Mininet inside Docker:


## General Commands Inside Mininet
- verify node: mininet> h1 ps
- inject rumor: mininet> h1 curl "http://10.0.0.1:9080/inject?id=r42&body=hello&ttl=8"
- view log: mininet> h1 tail -f /tmp/gossip-1.jsonl
- metric endpoint: mininet> h2 curl http://10.0.0.2:9080/metrics
- exit: mininet> exit


## Docker Dependencies



