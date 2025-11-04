package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type Coordinator struct {
	// Your definitions here.
	Mu                   sync.Mutex
	CompletedMapTasks    int
	CompletedReduceTasks int
	TotalMapTasks        int
	TotalReduceTasks     int
	MapTasks             map[int]*MapTask
	ReduceTasks          map[int]*ReduceTask
}

type MapTask struct {
	TaskId    int
	Filename  string
	Status    string
	StartTime time.Time
}

type ReduceTask struct {
	TaskId    int
	Status    string
	StartTime time.Time
}

// Your code here -- RPC handlers for the worker to call.
func (c *Coordinator) GetTask(args *TaskArgs, reply *TaskReply) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	switch {
	case c.CompletedMapTasks < c.TotalMapTasks:
		for _, task := range c.MapTasks {
			if task.Status == "Pending" {
				task.Status = "In Progress"
				task.StartTime = time.Now()
				reply.TaskType = "Map"
				reply.TaskId = task.TaskId
				reply.Filename = task.Filename
				reply.TotalMapTasks = c.TotalMapTasks
				reply.TotalReduceTasks = c.TotalReduceTasks
				return nil
			}
		}
		reply.TaskType = "Wait"
		return nil
	case c.CompletedReduceTasks < c.TotalReduceTasks:
		for _, task := range c.ReduceTasks {
			if task.Status == "Pending" {
				task.Status = "In Progress"
				task.StartTime = time.Now()
				reply.TaskType = "Reduce"
				reply.TaskId = task.TaskId
				reply.TotalMapTasks = c.TotalMapTasks
				reply.TotalReduceTasks = c.TotalReduceTasks
				return nil
			}
		}
		reply.TaskType = "Wait"
		return nil
	case c.CompletedMapTasks == c.TotalMapTasks && c.CompletedReduceTasks == c.TotalReduceTasks:
		reply.TaskType = "Done"
		return nil
	default:
		panic("Should not get here")
	}
}

func (c *Coordinator) CompleteTask(args *TaskArgs, reply *TaskReply) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	switch args.TaskType {
	case "Map":
		if c.MapTasks[args.TaskId].Status == "Completed" {
			return nil
		}
		c.MapTasks[args.TaskId].Status = "Completed"
		c.CompletedMapTasks++
	case "Reduce":
		if c.ReduceTasks[args.TaskId].Status == "Completed" {
			return nil
		}
		c.ReduceTasks[args.TaskId].Status = "Completed"
		c.CompletedReduceTasks++
	default:
		log.Printf("Unknown task type: %s", args.TaskType)
		panic("Should not get here")
	}
	return nil
}

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

// start a thread that listens for RPCs from worker.go
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

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if c.CompletedMapTasks == c.TotalMapTasks && c.CompletedReduceTasks == c.TotalReduceTasks {
		return true
	}
	return false
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		TotalMapTasks:    len(files),
		TotalReduceTasks: nReduce,
		MapTasks:         make(map[int]*MapTask),
		ReduceTasks:      make(map[int]*ReduceTask),
		Mu:               sync.Mutex{},
	}
	for i, file := range files {
		c.MapTasks[i] = &MapTask{
			TaskId:   i,
			Filename: file,
			Status:   "Pending",
		}
	}
	for i := 0; i < nReduce; i++ {
		c.ReduceTasks[i] = &ReduceTask{
			TaskId: i,
			Status: "Pending",
		}
	}
	go func() {
		for {
			c.Mu.Lock()
			for _, task := range c.MapTasks {
				if task.Status == "In Progress" && time.Since(task.StartTime) > 10*time.Second {
					task.Status = "Pending"
				}
			}
			for _, task := range c.ReduceTasks {
				if task.Status == "In Progress" && time.Since(task.StartTime) > 10*time.Second {
					task.Status = "Pending"
				}
			}
			c.Mu.Unlock()
			time.Sleep(1000 * time.Millisecond)
		}
	}()
	c.server()
	return &c
}
