## as2: Distributed MapReduce

- The master tells the worker about a new task by sending it the RPC call Worker.DoTask, giving a DoTaskArgs object as the RPC argument.

- The master may have to wait for a worker to finish before it can hand out more tasks. You may find channels useful to synchronize threads that are waiting for reply with the master once the reply arrives. Channels are explained in the document on Concurrency in Go.


- mr.registerChannel: new worker args
- mr.workers: string array of workers


- ok := call(mr.address, "Master.{FUNCNAME}", &args, &reply)

- STEP1: grab woker from channel
- STEP2: send worker an RPC to DoTask
- STEP3: only after success, return worker to channel, otherwise, retry by looping again
- STEP4: after all tasks in phase are done, return



