package mapreduce

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort" // to organize keys
)

// doReduce does the job of a reduce worker: it reads the intermediate
// key/value pairs (produced by the map phase) for this task, sorts the
// intermediate key/value pairs by key, calls the user-defined reduce function
// (reduceF) for each key, and writes the output to disk.
func doReduce(
	jobName string, // the name of the whole MapReduce job
	reduceTaskNumber int, // which reduce task this is
	nMap int, // the number of map tasks that were run ("M" in the paper)
	reduceF func(key string, values []string) string,
) {

	fmt.Printf("[DOREDUCE #%d] starting\n", reduceTaskNumber)

	// initializing buckets so that i can do this: bucket[key]=value
	buckets := make(map[string][]string)

	// for every map that was done
	for m := 0; m < nMap; m++ {
		// STEP 1: open all intermediate files produced by every map task decode JSON KV records
		rname := reduceName(jobName, m, reduceTaskNumber)
		f, err := os.Open(rname)
		if err != nil {
			fmt.Printf("[DOREDUCE] error opening file [%s]: %v\n", rname, err)
		}

		dec := json.NewDecoder(f)

		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err == io.EOF {
				break
			} else if err != nil {
				fmt.Printf("[DOREDUCE] couldn't decode %s: %v\n", rname, err)
				break // stop decoding
			}

			// STEP 2: build a bucket map by appending each record’s value under its key
			buckets[kv.Key] = append(buckets[kv.Key], kv.Value)
		}
		_ = f.Close() // we gotta close what we open :)
	}

	// STEP 3: collect all keys from buckets and sort them
	keys := make([]string, 0, len(buckets)) // make a slice that can hold all buckets aka all keys
	for k := range buckets {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	// initialize the output file here
	outname := mergeName(jobName, reduceTaskNumber)
	outfile, err := os.Create(outname)
	if err != nil {
		fmt.Printf("[DOREDUCE] couldn't create %s: %v\n", outname, err)
	}
	defer outfile.Close()

	enc := json.NewEncoder(outfile)

	// STEP 4: for each key in order, call reduceF to get the aggregated value
	for _, k := range keys {
		v := reduceF(k, buckets[k]) //

		// STEP 5: JSON-encode to this reducer’s output file
		enc.Encode(&KeyValue{Key: k, Value: v})
	}
	fmt.Printf("[DOREDUCE # %d] successfully completed\n", reduceTaskNumber)
}
