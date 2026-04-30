package executor

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"

	Types "github.com/sourabh945/ForgeQueue/Orchestrator/internal/types"
)

type Process struct {
	*Types.Process
}

// KillProcess kills the process with the given status code,
// if the status code is not -ve so the process is killed by orchestrator due some reason not due to some error.
// Like to many process is running and it have nothing have to process
func (proc *Process) KillProcess(exitCode int) {
	logger := proc.Logger.With(slog.String("type", "module"), slog.String("module", "process.KillProcess"))
	proc.ExitCode = exitCode
	err := proc.Cmd.Process.Kill()
	if err != nil {
		logger.Error("failed to kill process", slog.Any("error", err))
	}
	logger.Info("process killed successfully", slog.Int("statusCode", exitCode))

}

// WaitForProcess waits for the process to exit and sets the exit code accordingly.
// If process unable to get the exit code, then it set the exit code to -5.
func (proc *Process) WaitForProcess(cancelFxn context.CancelFunc) {
	defer cancelFxn()

	logger := proc.Logger.With(slog.String("type", "module"), slog.String("module", "process.WaitForProcess"))
	err := proc.Cmd.Wait()
	if proc.ExitCode == 256 {
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
