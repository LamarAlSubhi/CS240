# CS240

## as1: Sequential MapReduce
notes for myself:

steps for map:
1) it reads one of the input files (inFile)
2) calls the user-defined map function (mapF) for that file's contents
3) partitions the output into nReduce intermediate files.