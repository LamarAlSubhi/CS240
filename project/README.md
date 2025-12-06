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

## What is stored per rumor:
- Rumor ID: Globally unique identifier for the rumor (e.g., "g1-3").
- Payload / body: The actual message contents as a byte slice.
- TTL (time-to-live): Hop budget remaining; decremented on every push; used to limit spread.
- Origin node: ID of the node that first injected the rumor into the system.
- Injection timestamp: Time at which the origin node injected the rumor (Unix time in nanoseconds).
- Size: Payload size in bytes (for message cost / bandwidth analysis).

## What is stored per node:

#### Node identity & configuration
- Node ID (e.g., "g1").
- Gossip parameters (period, fanout, TTL).
- Anti-entropy interval.
- List of peers (seed-based, dynamic view of cluster).

#### Rumor state (the Store)
- Map of seen rumor IDs → full Rumor metadata (ID, TTL, Body, Origin, InjectTS, …).
- Set/list of newly added rumor IDs that are ready to be pushed in the next gossip tick.
- Derived stats like Digest (list of all rumor IDs known at this node).

#### Logging / instrumentation
- JSONL log file path.
- For each event (e.g., inject, deliver, optionally send/recv):
    - Event timestamp.
    - Node ID.
    - Event type.
    - Rumor ID.
    - Payload size.
    - Origin + injection timestamp (copied from rumor).

## What do we need to analyze
### Convergence properties (time to spread a rumor)

For convergence vs N, f, loss, etc., we need:

#### Injection time per rumor
- From log events where ev == "inject", grouped by id.
- This gives the start time of the rumor’s life.
#### First-delivery time per node, per rumor
- From log events where ev == "deliver", grouped by (id, node).
- This gives when each node first learned that rumor.
#### Convergence time per rumor
- For each rumor id:
    - t_inject(id) = min ts where ev=="inject" and id==...
    - For every node that ever sees it, t_first(node, id) = min ts where ev=="deliver" and id==...
    - Convergence time = max_node t_first(node, id) - t_inject(id)

### Plots:

#### Convergence Time vs Number of Nodes (N)

For a fixed fanout, period, TTL:
- Vary N = {5, 10, 20, 30, 40, 50}
- Inject one rumor
- Compute convergence time for each run
- Plot:
    - x-axis: number of nodes
    - y-axis: average convergence time (ms)
    - error bars: stddev over multiple runs

#### Convergence Time vs Loss Rate

Fix N; vary packet loss:
- Loss = {0%, 10%, 20%, 30%, 50%}
- Measure convergence time as above
- Plot:
    - x-axis: loss rate
    - y-axis: convergence time

#### Convergence Time vs Fanout (f)

Fix N; vary fanout:
- f = {1, 2, 3, 4, logN}
- Plot:
    - x-axis: fanout
    - y-axis: convergence time

#### Message Overhead vs Fanout (f)
- f = {1, 2, 3, 4, logN}
- Plot:
    - x-axis: fanout
    - y-axis: total messages sent (or bytes sent)

#### CDF of Delivery Times (per rumor)

For one rumor:
- Collect t_first(node, id) - t_inject(id)
- Plot CDF

#### Convergence Time vs Jitter

For jitter = {0ms, 5ms, 10ms, 20ms, 40ms}:
- Run gossip
- Plot convergence time



#### Convergence Time Under Churn (fail/rejoin)
During gossip:
- Drop k nodes at t = X
- Re-add them after Δ
- Plot convergence time vs churn rate

#### Message Count Over Time (Optional but great)

- Plot:
    - x-axis: time
    - y-axis: send or deliver events (count per second)


#### Delivery Percentage vs Time

- At each t, count % of nodes that have delivered the rumor
- Plot % delivered over time


#### Anti-Entropy Repair Time (BONUS)

If loss is high:
- Count number of rumors missing before repair
- Plot repair time to correctness
