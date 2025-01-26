package mr

import (
    "fmt"
    "log"
    "os"
    "net/rpc"
    "hash/fnv"
    "time"
    "encoding/json"
    "6.5840/logger"
)


//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	workerID := os.Getpid() // Fetch the unique process ID for this worker.
	logger.LogWithFuncName(fmt.Sprintf("[Worker] Worker ID: %d", workerID))
	for {
	    taskRequest := TaskRequest{WorkerID: workerID}
	    taskResponse := TaskResponse{}
	    logger.LogWithFuncName("[Worker] Requesting task from the coordinator")
	    ok := call("Coordinator.AssignTask", &taskRequest, &taskResponse)
        if ok {
            message := fmt.Sprintf("[Worker] Task received from the coordinator: %+v", taskResponse)
            logger.LogWithFuncName(message)
            //todo: filename empty
        } else {
            fmt.Println("Coordinator is unavailable")
            return
        }

        switch taskResponse.TaskType {
            case "map":
                performMapTask(mapf, taskResponse)
            case "reduce":
                performReduceTask(reducef, taskResponse)
            case "exit":
                log.Println("Worker exiting.")
                return
            default:
                time.Sleep(time.Second) // No tasks available, retry after 1 second
        }

        // Notify coordinator of task completion
        taskCompleted := TaskCompleted{
            TaskType: taskResponse.TaskType,
            TaskID:   taskResponse.TaskID,
            WorkerID: taskRequest.WorkerID,
        }
        call("Coordinator.TaskCompleted", &taskCompleted, nil)
	}

	// uncomment to send the Example RPC to the coordinator.
    // 	CallExample()

}

func performMapTask(mapf func(string, string) []KeyValue, taskResponse TaskResponse) {
    message := fmt.Sprintf("[Worker] Performing map task %d", taskResponse.TaskID)
    logger.LogWithFuncName(message)
    content, err := os.ReadFile(taskResponse.Filename)
    if err != nil {
    		log.Fatalf("Cannot read file %v: %v", taskResponse.Filename, err)
    }

    // Apply the map function
    intermediate := mapf(taskResponse.Filename, string(content))
    // Create nReduce empty intermediate files
    intermediateFiles := make([]*os.File, taskResponse.NReduce)
    encoders := make([]*json.Encoder, taskResponse.NReduce)
    for i := 0; i < taskResponse.NReduce; i++ {
        filename := fmt.Sprintf("mr-%v-%v", taskResponse.TaskID, i)// "mr-<mapTaskID>-<reducePartition>"
        file, err := os.Create(filename)
        if err != nil {
            log.Fatalf("Cannot create empty intermediate file %v: %v", filename, err)
        }
        intermediateFiles[i] = file
        encoders[i] = json.NewEncoder(file)
    }

    // Partition intermediate key/value pairs into nReduce files
	for _, kv := range intermediate {
		reduceIndex := ihash(kv.Key) % taskResponse.NReduce // Determine which reduce file to use
		err := encoders[reduceIndex].Encode(&kv)   // Encode key-value pair as JSON
		if err != nil {
			log.Fatalf("Error encoding key-value pair: %v", err)
		}
	}

	// Close all intermediate files
	for _, file := range intermediateFiles {
		file.Close()
	}

}

func performReduceTask(reducef func(string, []string) string, taskResponse TaskResponse) {
	// Collect intermediate key-value pairs from all map tasks
	message := fmt.Sprintf("[Worker] Performing reduce task %d", taskResponse.TaskID)
    logger.LogWithFuncName(message)
	intermediate := make(map[string][]string) //a map of slices, ex. m["fruits"] = []string{"apple", "banana", "cherry"}
	for i := 0; i < taskResponse.NMap; i++ {
		filename := fmt.Sprintf("mr-%d-%d", i, taskResponse.TaskID) // "mr-<mapTaskID>-<reduceTaskID>"
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("Cannot open intermediate file %v: %v", filename, err)
		}

		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break // End of file
			}
			intermediate[kv.Key] = append(intermediate[kv.Key], kv.Value) // Group values by key
		}
		file.Close()
	}

	// Create the final output file for this reduce task mr-out-<reduceTaskID>
	oname := fmt.Sprintf("mr-out-%d", taskResponse.TaskID)
	ofile, err := os.Create(oname)
	if err != nil {
		log.Fatalf("Cannot create output file %v: %v", oname, err)
	}

	// Apply the reduce function and write the result
	for key, values := range intermediate {
		output := reducef(key, values) // Merge values for each key
		fmt.Fprintf(ofile, "%v %v\n", key, output) // Write in the required format
	}
	ofile.Close()
}


//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
