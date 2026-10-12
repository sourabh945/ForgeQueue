package types

import (
	"io"
	"net"
	"os/exec"
	"sync"
)

type Status string

const (
	Free     Status = "free"
	Running  Status = "running"
	Complete Status = "complete"
	Failed   Status = "stopped"
)

type Worker struct {
	Mutex       sync.RWMutex  // Mutex for thread safety
	ID          string        // Unqiue identifier for the worker process
	Status      Status        // Status: running, free, failed, completed
	SocketPath  string        // Path of the unix domain socket
	Cmd         *exec.Cmd     // process executor pointer
	Conn        net.Conn      // ipc socket connection
	StdOutPipe  io.ReadCloser // stdout pipe of worker process for logging
	StdErrPipe  io.ReadCloser // stderr pipe of worker process for logging
	Job         *Task         // Running task of the worker
	JobResponse *TaskResult   // Result of the running task of the worker
}
