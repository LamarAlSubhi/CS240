package mapreduce

// schedule starts and waits for all tasks in the given phase (Map or Reduce).
func (mr *Master) schedule(phase jobPhase) {
	var ntasks int
	var nios int // number of inputs (for reduce) or outputs (for map)
	switch phase {
	case mapPhase:
		ntasks = len(mr.files)
		nios = mr.nReduce
	case reducePhase:
		ntasks = mr.nReduce
		nios = len(mr.files)
	}

	// im making a channel that will be populated with true for every done task
	doneChan := make(chan bool, ntasks)

	file := "" // initializing file arg for DoTaskArgs struct

	debug("Schedule: %v %v tasks (%d I/Os)\n", ntasks, phase, nios)

	for task := 0; task < ntasks; task++ {
		task := task // rebind loop var so that the go routines get correct task

		go func() {

			// setting up DoTaskArgs
			if phase == mapPhase {
				// use the task number as index for files
				file = mr.files[task]
			}
			args := DoTaskArgs{JobName: mr.jobName, File: file, Phase: phase, TaskNumber: task, NumOtherPhase: nios}
			reply := new(struct{}) // just emptyx

			for {
				// 	STEP1: grab woker from channel
				worker := <-mr.registerChannel

				// STEP2: send worker an RPC to DoTask
				debug("calling worker %v\n", worker)
				ok := call(worker, "Worker.DoTask", &args, reply)

				if ok {
					// STEP3: only after success, return worker to channel
					go func() { mr.registerChannel <- worker }()
					doneChan <- true
					return
				}
				// otherwise, retry by looping again
				debug("worker %v failed task %v, retrying...\n", worker, args.TaskNumber)
			}
		}()
	}

	// STEP4: after all tasks in phase are done, return
	// outside for loop and go routines
	for i := 0; i < ntasks; i++ {
		<-doneChan
	}

	debug("Schedule: %v phase done\n", phase)
}
