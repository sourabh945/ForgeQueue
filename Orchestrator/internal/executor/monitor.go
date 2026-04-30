package executor

import (
	"bufio"
	"log/slog"
)

func _catchErr(err error, logger *slog.Logger) {
	if err == bufio.ErrTooLong {
		logger.Error("line too long, increase buffer size")
	} else {
		logger.Error("scanner error", slog.Any("err", err))
	}

}

// StdOutLogger logs the stdout stream of the process
func (proc *Process) StdOutLogger() {
	stdoutLogger := proc.Logger.With(slog.String("type", "stream"), slog.String("stream", "stdout"))
	moduleLogger := proc.Logger.With(slog.String("type", "module"), slog.String("module", "monitor.StdOutLogger"))
	scanner := bufio.NewScanner(proc.StdOut)
	for scanner.Scan() {
		stdoutLogger.Info("stdout: " + scanner.Text())
	}
	err := scanner.Err()
	_catchErr(err, moduleLogger)
}

// StdErrLogger logs the stderr stream of the process
func (proc *Process) StdErrLogger() {
	stderrLogger := proc.Logger.With(slog.String("type", "stream"), slog.String("stream", "stderr"))
	moduleLogger := proc.Logger.With(slog.String("type", "module"), slog.String("module", "monitor.StdErrLogger"))
	scanner := bufio.NewScanner(proc.StdErr)
	for scanner.Scan() {
		stderrLogger.Error("stderr: " + scanner.Text())
	}
	err := scanner.Err()
	_catchErr(err, moduleLogger)
}
