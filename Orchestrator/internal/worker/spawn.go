package worker

import (
	"fmt"
	"io"
	"os/exec"
	"syscall"
)

// SpawnWorkerProcess is for spawn a new process for the worker
// It configures a unique Process Group ID (PGID) to allow clean cascading terminations
func SpawnWorkerProcess(pythonBinPath string, workerFilePath string, socketPath string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {

	// arguments for the worker process to start
	cmd := exec.Command(pythonBinPath, workerFilePath, socketPath)

	// Force the child process to start in its own new process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,            //own process group (enables KillGroup/TermGroup)
		Pdeathsig: syscall.SIGKILL, // die if orchestrator dies (Linux only)
	}

	// getting stdout and stderr pipe
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("worker.SpawnWorkerProcess: stdoutPipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("worker.SpawnWorkerProcess: stderrPipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("worker.SpawnWorker: %w", err)
	}

	return cmd, stdoutPipe, stderrPipe, nil
}

// TermGroup kill all the group process of the worker using SIGTERM
// Its for make sure all group of the process ( worker + worker child process ) are killed
func TermGroup(cmd *exec.Cmd) {
	syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

// KillGroup kill all the group process of the worker using SIGKILL
// Its for make sure all group of the process ( worker + worker child process ) are killed
// It uses the SIGKILL so if the worker not allowed to terminate it will do using the OS inforcer
func KillGroup(cmd *exec.Cmd) {
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
