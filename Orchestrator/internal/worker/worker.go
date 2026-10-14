package worker

import (
	"fmt"
	"log/slog"
	"orch/internal/ipc"
	"orch/internal/types"
	"orch/internal/utils"
	"os"
	"path/filepath"
	"time"
)

const (
	// WorkerStartTimeout is the total time for: process start + socket connect + HELO.
	WorkerStartTimeout = 10 * time.Second
)

// NewWorker create a new Worker object
// The worker is returned even when the start fails (status dead) so the caller can retry StartWorker on it
func NewWorker(id string, socketDir string, pythonBinPath string, scriptPath string, stdStreamLogger *slog.Logger) (*types.Worker, error) {
	socketPath := fmt.Sprintf("%v/temp-%v.sock", socketDir, id)

	worker := &types.Worker{
		ID:         id,
		SocketPath: socketPath,
		Status:     types.WorkerStarting,
	}

	if err := StartWorker(worker, pythonBinPath, scriptPath, WorkerStartTimeout, stdStreamLogger); err != nil {
		return worker, fmt.Errorf("worker.NewWorker: %w", err)
	}

	return worker, nil
}

// StartWorker is actually start the worker process
// and IPC for the worker and make those change to the worker
func StartWorker(w *types.Worker, pythonBinPath string, scriptPath string, startTimeout time.Duration, stdStreamLogger *slog.Logger) error {

	errh := utils.ErrorHandler{
		Pkg: "worker",
		Fxn: "StartWorker",
	}

	// one deadline for the whole start-up: spawn + connect + HELO
	deadline := time.Now().Add(startTimeout)

	// creating + check exists the socket directory before the listener is created
	if err := os.MkdirAll(filepath.Dir(w.SocketPath), 0o700); err != nil {
		w.SetStatus(types.WorkerDead)
		return errh.Errorf("socket dir: %w", err)
	}

	w.Mutex.Lock()
	w.Status = types.WorkerStarting
	w.StartedAt = time.Now()
	w.Mutex.Unlock()

	ls, err := ipc.CreateListener(w.SocketPath)
	if err != nil {
		w.SetStatus(types.WorkerDead)
		return errh.Perror(err)
	}

	// closing the listener at the end of the functions
	defer ls.Close()

	cmd, stdoutPipe, stderrPipe, err := SpawnWorkerProcess(pythonBinPath, scriptPath, w.SocketPath)
	if err != nil {
		w.SetStatus(types.WorkerDead)
		return errh.Perror(err)
	}

	// recieve-only channel produced by the WatchProcess signaled at process end
	exited := WatchProcess(w.ID, cmd, stdoutPipe, stderrPipe, stdStreamLogger)

	startDone := make(chan struct{})
	defer close(startDone)

	// making sure the listener is closed when the process exists before connection accepted
	go func() {
		select {
		case <-exited:
			ls.Close()
		case <-startDone:
		}
	}()

	// process cleanup function if process failed
	fail := func(err error) error {
		w.KillGroup()
		w.SetStatus(types.WorkerDead)
		return errh.Perror(err)
	}

	acceptWait := time.Until(deadline)
	conn, err := ipc.AcceptWorker(ls, &acceptWait)
	if err != nil {
		select {
		case <-exited:
			err = fmt.Errorf("worker process exited before connected: %w", err)
		default:
		}
		return fail(err)
	}

	heloWait := time.Until(deadline)
	if err := ipc.WaitForHelo(conn, &heloWait); err != nil {
		conn.Close()
		return fail(err)
	}

	w.Mutex.Lock()
	w.Cmd = cmd
	w.Conn = conn
	w.StdOutPipe = stdoutPipe
	w.StdErrPipe = stderrPipe
	w.Exited = exited
	w.Status = types.WorkerFree
	w.Mutex.Unlock()

	return nil
}
