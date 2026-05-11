package types

import (
	"io"
	"log/slog"
	"net"
	"os/exec"
	"sync"
)

type Worker struct {
	ConnMu      sync.RWMutex  // Mutex for read/write operations on the Conn, multiple readers and single writer
	ID          string        // It is unique identifier for the process
	Status      string        // It is status of the worker is : running, free, frezzed, stopped, fail
	SocketPath  string        // It is path to the unix socket
	Cmd         *exec.Cmd     // It is process executor pointer
	Conn        net.Conn      // It is connection to the unix socket
	StdOut      io.ReadCloser // It is stdout Pipe of the process
	StdErr      io.ReadCloser // It is stderr Pipe of the process
	Logger      *slog.Logger  // It is logger for logging the process execution
	Job         *Job          // It is job for the process to execute
	JobResponse *JobResponse  // It is response from the process
	ExitCode    int           // It is exit code of the process 0 for success, 1-255 for failure or -ve for intensional kill and 256 for no value
}

type WorkerProcessConfig struct {
	MaxTime int // It is maximum time in seconds for the process to execute after which it will be terminated
}
