package executor

import (
	"bufio"
	"log/slog"
)

func catchErr(err error, logger *slog.Logger) {
	if err == bufio.ErrTooLong {
		logger.Error("line too long, increase buffer size")
	} else {
		logger.Error("scanner error", slog.Any("err", err))
	}

}

func (proc *Process) StdOutLogger() {
	stdoutLogger := proc.Logger.With("logger", "stdout")
	scanner := bufio.NewScanner(proc.StdOut)
	for scanner.Scan() {
		stdoutLogger.Info("stdout: " + scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		proc.Logger.Error("stdout scanner error: " + err.Error())
	}
}
func (proc *Process) StdErrLogger() {
	scanner := bufio.NewScanner(proc.StdErr)
	for scanner.Scan() {
		proc.Logger.Info("stderr: " + scanner.Text())
	}
}
