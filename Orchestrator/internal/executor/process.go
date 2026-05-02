package executor

import (
	"errors"
	"log/slog"
	"os/exec"
	"syscall"

	Types "github.com/sourabh945/ForgeQueue/Orchestrator/internal/types"
)

type Worker struct {
	*Types.Worker
}

// KillProcess kills the process with the given status code,
// if the status code is not -ve so the process is killed by orchestrator due some reason not due to some error.
// Like to many process is running and it have nothing have to process
func (proc *Worker) KillProcess(exitCode int) {
	logger := proc.Logger.With(slog.String("type", "module"), slog.String("module", "worker.KillProcess"))
	proc.ExitCode = exitCode
	err := proc.Cmd.Process.Kill()
	if err != nil {
		logger.Error("failed to kill process", slog.Any("error", err))
	}
	logger.Info("process killed successfully", slog.Int("exitCode", exitCode))

}

// WaitForProcess waits for the process to exit and sets the exit code accordingly.
// If process unable to get the exit code, then it set the exit code to -5.
func (proc *Worker) WaitForProcess() {

	logger := proc.Logger.With(slog.String("type", "module"), slog.String("module", "worker.WaitForProcess"))
	err := proc.Cmd.Wait()
	if proc.ExitCode == 256 || proc.ExitCode > 0 {
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				proc.ExitCode = exitErr.ExitCode()
			} else {
				proc.ExitCode = -5
			}
		}
		logger.Error("process exited", slog.Any("exitCode", proc.ExitCode))
	}

}

// function FreezeProcess freezes the process, it is used to stop the process without killing it.
func (proc *Worker) FreezeProcess() {
	logger := proc.Logger.With(slog.String("type", "module"), slog.String("module", "worker.FreezeProcess"))
	logger.Info("Start frezzing the worker")
	proc.Cmd.Process.Signal(syscall.SIGSTOP)
	logger.Info("Worker frozen successfully")
}

// function UnfreezeProcess unfreezes the process, it is used to resume the process after it has been frozen.
func (proc *Worker) UnfreezeProcess() {
	logger := proc.Logger.With(slog.String("type", "module"), slog.String("module", "worker.UnfreezeProcess"))
	logger.Info("Start unfreezing the worker")
	proc.Cmd.Process.Signal(syscall.SIGCONT)
	logger.Info("Worker unfrozen successfully")
}
