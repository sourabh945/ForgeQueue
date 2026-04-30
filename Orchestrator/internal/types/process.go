package types

import (
	"io"
	"log/slog"
	"os/exec"
)

type Process struct {
	Cmd      *exec.Cmd     // It is process executor pointer
	StdOut   io.ReadCloser // It is stdout Pipe of the process
	StdErr   io.ReadCloser // It is stderr Pipe of the process
	Logger   *slog.Logger  // It is logger for logging the process execution
	Job      *Job          // It is job for the process to execute
	ExitCode int           // It is exit code of the process 0 for success, 1-255 for failure or -ve for intensional kill and 256 for no value
}

type ProcessConfig struct {
	MaxTime int // It is maximum time in seconds for the process to execute after which it will be terminated
}
