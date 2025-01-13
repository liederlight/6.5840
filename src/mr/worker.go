package mr

import "fmt"
import "log"
import "net/rpc"
import "hash/fnv"


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

	for (
	    taskRequest := TaskRequest{}
	    taskResponse := TaskResponse{}
	    taskResponse := call("Coordinator.AssignTask", &taskRequest, &taskResponse)
        if ok {
            fmt.Println("Task received:", taskResponse)
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
	)

	// Your worker implementation here.

	// uncomment to send the Example RPC to the coordinator.
// 	CallExample()

}

func performMapTask(mapf func(string, string) []KeyValue, taskResponse TaskResponse) {
    fmt.Println("Performing map task", taskResponse.TaskID)
    // Read input file content
    content, err := ioutil.ReadFile(task.Filename)
    if err != nil {
    		log.Fatalf("Cannot read file %v: %v", task.Filename, err)
    }

    // Apply the map function
    intermediate := mapf(task.Filename, string(content))
    // Create nReduce empty intermediate files
    intermediateFiles := make([]*os.File, task.NReduce)
    encoders := make([]*json.Encoder, task.NReduce)
    for i := 0; i < task.NReduce; i++ {
        filename := fmt.Sprintf("mr-%v-%v", task.TaskID, i)
        file, err := os.Create(filename)
        if err != nil {
            log.Fatalf("Cannot create empty intermediate file %v: %v", filename, err)
        }
        intermediateFiles[i] = file
        encoders[i] = json.NewEncoder(file)
    }

    // Partition intermediate key/value pairs into nReduce files
	for _, kv := range intermediate {
		reduceIndex := ihash(kv.Key) % task.NReduce // Determine which reduce file to use
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

func performReduceTask(reducef func(string, []string) string, task TaskResponse) {
	// Collect intermediate key-value pairs from all map tasks
	intermediate := make(map[string][]string) //a map of slices
	for i := 0; i < task.NMap; i++ {
		filename := fmt.Sprintf("mr-%d-%d", i, task.TaskID) // Format: "mr-X-Y"
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

	// Create the final output file for this reduce task
	oname := fmt.Sprintf("mr-out-%d", task.TaskID)
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
