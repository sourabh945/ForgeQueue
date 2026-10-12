package worker

import (
	"fmt"
	"os/exec"
	"syscall"
)

// SpawnWorker is for spawn a new process for the worker
// It configures a unique Process Group ID (PGID) to allow clean cascading terminations
func SpawnWorker(workerFilePath string, socketPath string) (*exec.Cmd, error) {

	// arguments for the worker process to start
	args := fmt.Sprintf("%v %v", workerFilePath, socketPath)

	cmd := exec.Command("python3", args)

	// Force the child process to start in its own new process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("worker.SpawnWorker: %w", err)
	}

	return cmd, nil
}

//
