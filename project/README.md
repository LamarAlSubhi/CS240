# Project 
Prof suggested changes:

- run gossip protocols across separate processes rather than within a single simulator. 
- deploy the nodes inside a Mininet environment and experiment with controllable delay, bandwidth, and churn.


## Workflow? / plan? / something?

1. Message format + basic push on reliable links(DONE)
2. Push-pull + TTL (DONE)
3. Anti-entropy reconciliation (DONE)
4. Mininet bring-up (DONE)
5. Baseline experiments (convergence vs N, f) in Mininet (Done)
6. Loss, jitter, bandwidth limits, and churn (33.6% Done)
7. Analysis & plots (13.4% Done)
8. Report & slides

## General Commands Inside Mininet
- verify node: mininet> h1 ps
- inject rumor: mininet> h1 curl "http://10.0.0.1:9080/inject?id=r42&body=hello&ttl=8"
- view log: mininet> h1 tail -f /tmp/gossip-1.jsonl
- metric endpoint: mininet> h2 curl http://10.0.0.2:9080/metrics
- exit: mininet> exit

## Dependencies

Ubuntu 22.04+ includes Mininet in its official repositories.
Do NOT use Docker images or the Mininet GitHub install script (it fails on Python 3.12 / PEP 668).
- sudo apt update
- sudo apt install -y mininet python3 python3-pip python3-venv git


