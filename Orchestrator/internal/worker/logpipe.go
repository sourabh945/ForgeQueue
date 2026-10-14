package worker

import (
	"bufio"
	"io"
	"log"
	"log/slog"
	"os/exec"
	"sync"
)

// WatchProcess does the two jobs that must always happen for a running worker process:
// 1. read everything it write to stdout and stderr.
// 2. call cmd.Wait() once the process is gone, to collect the exit status and remove the zombie
//
// The returned channel is closed whe the process has exited.
func WatchProcess(workerId string, cmd *exec.Cmd, stdoutPipe io.Reader, stderrPipe io.Reader, stdStreamLogger *slog.Logger) <-chan struct{} {

	var readers sync.WaitGroup
	readers.Add(2)

	// setting up logger
	_logger := stdStreamLogger.With(slog.String("ID", workerId))

	// start the goroutine for pipes
	go pipeLogs(stdoutPipe, _logger, "stdout", &readers)
	go pipeLogs(stderrPipe, _logger, "stderr", &readers)

	exited := make(chan struct{})
	go func() {
		readers.Wait()
		err := cmd.Wait()
		if err != nil {
			log.Printf("[workerId: %s] worker.WatchProcess: %w", workerId, err)
		}
		log.Println("[workerId: %s] worker.WatchProcess: worker is dead")
		close(exited)
	}()

	return exited

}

// pipeLogs forward every line the worker write sto the a logs stream to a log file
func pipeLogs(r io.Reader, l *slog.Logger, stream string, wg *sync.WaitGroup) {
	defer wg.Done()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // setting the buffer size of 64 KiB (buffer size of linux processes)
	if stream == "stderr" {
		for sc.Scan() {
			l.Error(sc.Text())
		}
	} else {
		for sc.Scan() {
			l.Info(sc.Text())
		}
	}
	io.Copy(io.Discard, r)
}
