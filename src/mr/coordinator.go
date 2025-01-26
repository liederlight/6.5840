package mr

import (
    "fmt"
    "log"
    "net"
    "os"
    "net/rpc"
    "net/http"
    "sync"
)

type TaskStatus int

const (
    Idle TaskStatus = iota
    InProgress
    Completed
)

type Coordinator struct {
	// Your definitions here.
	mu sync.Mutex
	mapTasks   []string // List of files to process

    nReduce int
    nMap int

	mapTaskStatus []TaskStatus
	reduceTaskStatus []TaskStatus

	mapDone bool
	reduceDone bool

	nRemainingMapTasks int
	nRemainingReduceTasks int
}

// Your code here -- RPC handlers for the worker to call.
func (c *Coordinator) AssignTask(args *TaskRequest, reply *TaskResponse) error {
    // Your code here.
    c.mu.Lock()
    defer c.mu.Unlock()
    if !c.mapDone {
        taskID := c.getIdleMapTask()
        if taskID != -1 { // Found an idle map task
            reply.TaskType = "map"
            reply.TaskID = taskID
            reply.Filename = c.mapTasks[taskID]
            reply.NReduce = c.nReduce
            reply.NMap = c.nMap
            c.mapTaskStatus[taskID] = InProgress
            return nil
        } else { // <-- 'else' must be on the same line as the closing brace of 'if'
            c.mapDone = true
        }
    }
    if !c.reduceDone {
        taskID := c.getIdleReduceTask()
        if taskID != -1 { // Found an idle reduce task
            reply.TaskType = "reduce"
            reply.TaskID = taskID
            reply.NReduce = c.nReduce
            reply.NMap = c.nMap
            c.reduceTaskStatus[taskID] = InProgress
            return nil
        } else {
            c.reduceDone = true
        }
    }
    reply.TaskType = "exit"
    return nil
}

func (c *Coordinator) getIdleMapTask() int {
    for i, status := range c.mapTaskStatus {
        if status == Idle {
            return i
        }
    }
    return -1
}

func (c *Coordinator) getIdleReduceTask() int {
    for i, status := range c.reduceTaskStatus {
        if status == Idle {
            return i
        }
    }
    return -1
}

// In Go’s built-in net/rpc system, each RPC handler must follow this signature form:
// func (receiver) MethodName(args *T, reply *U) error
func (c *Coordinator) TaskCompleted(args *TaskCompleted, reply *struct{}) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if args.TaskType == "map" {
        c.mapTaskStatus[args.TaskID] = Completed
        c.nRemainingMapTasks--
        if c.nRemainingMapTasks == 0 {
            c.mapDone = true
        }
    } else if args.TaskType == "reduce" {
        c.reduceTaskStatus[args.TaskID] = Completed
        c.nRemainingReduceTasks--
        if c.nRemainingReduceTasks == 0 {
            c.reduceDone = true
        }
    }
    return nil
}
//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}


//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Printf("Map Done: %v, Reduce Done: %v\n", c.mapDone, c.reduceDone)
	return c.mapDone && c.reduceDone
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}

	// Your code here.
	c.mapTasks = files
	c.nReduce = nReduce
	c.nMap = len(files)
	c.mapTaskStatus = make([]TaskStatus, c.nMap)
	c.reduceTaskStatus = make([]TaskStatus, c.nReduce)
	c.mapDone = false
	c.reduceDone = false
	c.nRemainingMapTasks = c.nMap
	c.nRemainingReduceTasks = c.nReduce

	fmt.Printf("Coordinator created as %+v\n", c)

	c.server()
	log.Printf("Coordinator server started\n")
	//todo: go c.monitorTimeouts()
	return &c
}
