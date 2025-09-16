# CS240

## as1: Sequential MapReduce
notes for myself:

- Partition rule: r = ihash(key) % nReduce so all identical keys go to the same reducer
- Intermediate files (per map task i): produce exactly nReduce files: f{i}-0, f{i}-1, …, f{i}-{nReduce-1}.

- intermediate files = JSON KV records (map → reduce)
- reduce outputs = plain text “key value” lines (reduce → final merge)

- All KVs with r = 0 go into the *-0 file for each map task
- Later, reduce task 0 will read all *-0 files across map tasks
- That way, reducer 0 sees every KV that hashed to bucket 0




## Map flow:

- STEP 1: read the whole input file for this task
- STEP 2: call the mapF on the content
- STEP 3: split the pairs produced by STEP 2 using their reduce index r = hash(key) % nReduce. 
- STEP 4: for each single reduce index r, I add the pairs that belong to r into the intermediate file for r. 

#### example:

## inputs:
- files: 
    -  input1.txt: "foo bar foo" 
    -  input2.txt: "bar baz"
- mapF (wc): returns (word, "1") for each word
- nReduce = 2 (so keys are split between r=0 and r=1)

## DoMap on input1.txt
- calls mapF: 
    - mapF("input1.txt", "foo bar foo") → [("foo","1"), ("bar","1"), ("foo","1")]

- partitions (using hash key):
    - "foo" → reducer 0
    - "bar" → reducer 1

- writes n intermediate files:
    - mr-0 (for reducer 0): {"Key":"foo","Value":"1"}{"Key":"foo","Value":"1"}
    - mr-1 (for reducer 1): {"Key":"bar","Value":"1"}

## DoMap on input2.txt
- calls mapF: 
    - mapF("input2.txt", "bar baz") → [("bar","1"), ("baz","1")]

- partitions (using hash key):
    - "baz" → reducer 0
    - "bar" → reducer 1

- writes n intermediate files (JSON objects, one per line):
    - mr-2-0 (for reducer 0): {"Key":"bar","Value":"1"}
    - mr-2-1 (for reducer 1): {"Key":"baz","Value":"1"}



## REDUCE LATER






