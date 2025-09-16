package mapreduce

import (
	"encoding/json" // for the json objects
	"fmt"           // for printing
	"hash/fnv"
	"os" // so i can read and make files and stuff
)

// doMap does the job of a map worker: it reads one of the input files
// (inFile), calls the user-defined map function (mapF) for that file's
// contents, and partitions the output into nReduce intermediate files.
func doMap(
	jobName string, // the name of the MapReduce job
	mapTaskNumber int, // which map task this is
	inFile string,
	nReduce int, // the number of reduce task that will be run ("R" in the paper)
	mapF func(file string, contents string) []KeyValue,
) {

	// STEP 1: read the whole input file for this task

	debug("[DOMAP #%d] starting", mapTaskNumber)

	bytes, err := os.ReadFile(inFile) // returns byte []

	if err != nil {
		fmt.Printf("[DOMAP]error reading file [%s]: %v\n", inFile, err)
		// im thorough, arent i ;)
	}
	content := string(bytes) // need as string for mapF

	debug("[DOMAP] read %d bytes from %s\n", len(content), inFile)

	// STEP 2: call the mapF on the content
	kvs := mapF(inFile, content)
	debug("[DOMAP] mapF produced %d kvs for %s\n", len(kvs), inFile)
	if len(kvs) > 0 {
		debug("[DOMAP] sample kv: %q -> %q\n", kvs[0].Key, kvs[0].Value)
	}

	// STEP 3: split the pairs produced by STEP 2 using their reduce index r = hash(key) % nReduce.

	// loop prep, create files
	files := make([]*os.File, nReduce) // one intermediate file per reducer index r for each map task
	encs := make([]*json.Encoder, nReduce)

	// lamda func to close the files at the end
	defer func() {
		for _, f := range files {
			if f != nil {
				_ = f.Close()
			}
		}
	}()

	for _, kv := range kvs {
		r := int(ihash(kv.Key)) % nReduce
		if encs[r] == nil {
			rname := reduceName(jobName, mapTaskNumber, r) // get file name
			f, err := os.Create(rname)                     // creating the actual file

			if err != nil {
				fmt.Printf("[DOMAP] error creating file [%s]: %v\n", rname, err)
			}

			files[r] = f
			encs[r] = json.NewEncoder(f)
		}
		// STEP 4: for each single reduce index r, I add the pairs that belong to r into the intermediate file for r.
		err := encs[r].Encode(&kv)

		if err != nil {
			fmt.Printf("[DOMAP] error encoding file %v\n", err)
		}
	}
	debug("[DOMAP #%d] successfully completed", mapTaskNumber)
}

func ihash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
