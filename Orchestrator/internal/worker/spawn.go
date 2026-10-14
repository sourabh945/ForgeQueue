package worker

import (
	"io"
	"orch/internal/utils"
	"os/exec"
	"syscall"
)

// SpawnWorkerProcess is for spawn a new process for the worker
// It configures a unique Process Group ID (PGID) to allow clean cascading terminations
func SpawnWorkerProcess(pythonBinPath string, workerFilePath string, socketPath string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {

	// define error handler
	errh := utils.ErrorHandler{
		Pkg: "worker",
		Fxn: "SpawnWorkerProcess",
	}

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
		return nil, nil, nil, errh.Errorf("stdoutPipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, errh.Errorf("stderrPipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, errh.Perror(err)
	}

	return cmd, stdoutPipe, stderrPipe, nil
}
