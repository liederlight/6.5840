package mr

import (
    "fmt"
    "log"
    "net"
    "os"
    "net/rpc"
    "net/http"
    "sync"
    "6.5840/logger"
    "time"
)

type TaskStatus int

const (
    Idle TaskStatus = iota
    InProgress
    Completed
)

type TaskInfo struct {
    Status    TaskStatus
    StartTime time.Time
}

type Coordinator struct {
	// Your definitions here.
	mu sync.Mutex
	mapTasks   []string // List of files to process

    nReduce int
    nMap int // todo: Number of files to process

	mapTaskStatus []TaskInfo
	reduceTaskStatus []TaskInfo

	mapDone bool
	reduceDone bool

	nRemainingMapTasks int
	nRemainingReduceTasks int
	timeout            time.Duration
}

// Your code here -- RPC handlers for the worker to call.
func (c *Coordinator) AssignTask(args *TaskRequest, reply *TaskResponse) error {
    // Your code here.
    c.mu.Lock()
    defer c.mu.Unlock()
    // If all map tasks are done, assign reduce tasks. Else, assign exit task before even assigning reduce tasks
    if !c.mapDone {
        taskID := c.getIdleMapTask()
        if taskID != -1 { // Found an idle map task
            reply.TaskType = "map"
            reply.TaskID = taskID
            reply.Filename = c.mapTasks[taskID]
            reply.NReduce = c.nReduce
            reply.NMap = c.nMap
            c.mapTaskStatus[taskID].Status = InProgress
            c.mapTaskStatus[taskID].StartTime = time.Now()
            message := fmt.Sprintf("[Coordinator] Assigning map task %d with file: %s to the worker ", taskID, c.mapTasks[taskID])
            logger.LogWithFuncName(message)
            return nil
        } else { // <-- 'else' must be on the same line as the closing brace of 'if'
            c.mapDone = true
        }
    }
    // If all map tasks are done, assign reduce tasks. Else, assign exit task before even assigning reduce tasks
    if !c.reduceDone {
        taskID := c.getIdleReduceTask()
        if taskID != -1 { // Found an idle reduce task
            reply.TaskType = "reduce"
            reply.TaskID = taskID
            reply.NReduce = c.nReduce
            reply.NMap = c.nMap
            c.reduceTaskStatus[taskID].Status = InProgress
            c.reduceTaskStatus[taskID].StartTime = time.Now()
            return nil
        } else {
            c.reduceDone = true
        }
    }
    reply.TaskType = "exit"
    return nil
}

func (c *Coordinator) getIdleMapTask() int {
    for i, info := range c.mapTaskStatus {
        if info.Status == Idle {
            return i
        }
        if info.Status == InProgress && time.Since(info.StartTime) > c.timeout {
            logger.LogWithFuncName(fmt.Sprintf("[Coordinator] Map task %d has timed out", i))
            return i
        }
    }
    return -1
}

func (c *Coordinator) getIdleReduceTask() int {
    for i, info := range c.reduceTaskStatus {
        if info.Status == Idle {
            return i
        }
        if info.Status == InProgress && time.Since(info.StartTime) > c.timeout {
            logger.LogWithFuncName(fmt.Sprintf("[Coordinator] Reduce task %d has timed out", i))
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
        c.mapTaskStatus[args.TaskID].Status = Completed
        c.nRemainingMapTasks--
        if c.nRemainingMapTasks == 0 {
            c.mapDone = true
        }
    } else if args.TaskType == "reduce" {
        c.reduceTaskStatus[args.TaskID].Status = Completed
        c.nRemainingReduceTasks--
        if c.nRemainingReduceTasks == 0 {
            c.reduceDone = true
        }
    }
    message := fmt.Sprintf("[Coordinator] Task %d of type %s completed by worker %d", args.TaskID, args.TaskType, args.WorkerID)
    logger.LogWithFuncName(message)
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
	message := fmt.Sprintf("[Coordinator] Checking if all tasks are done. Map Done: %v, Reduce Done: %v", c.mapDone, c.reduceDone)
	logger.LogWithFuncName(message)
// 	log.Printf("[Coordinator] Map Status: %v, Reduce Status: %v", c.mapTaskStatus, c.reduceTaskStatus)
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
	c.mapTaskStatus = make([]TaskInfo, c.nMap)
	c.reduceTaskStatus = make([]TaskInfo, c.nReduce)
	c.mapDone = false
	c.reduceDone = false
	c.nRemainingMapTasks = c.nMap
	c.nRemainingReduceTasks = c.nReduce
	c.timeout = 10 * time.Second
	for i := 0; i < c.nMap; i++ {
        c.mapTaskStatus[i] = TaskInfo{Status: Idle, StartTime: time.Now()}
    }
    for i := 0; i < c.nReduce; i++ {
        c.reduceTaskStatus[i] = TaskInfo{Status: Idle, StartTime: time.Now()}
    }

	fmt.Printf("Coordinator created as %+v\n", c)

	c.server()
	//todo: go c.monitorTimeouts()
	return &c
}
