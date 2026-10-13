package worker

import (
	"fmt"
	"orch/internal/ipc"
	types "orch/internal/types"
	"orch/internal/utils"
	"time"
)

const (
	WorkerStartTimeout = 2 * time.Second
)

// NewWorker create a new Worker object
func NewWorker(id string, socketDir string, pythonBinPath string, scriptPath string) (*types.Worker, error) {
	socketPath := fmt.Sprintf("%v/temp-%v.sock", socketDir, id)

	worker := &types.Worker{
		ID:         id,
		SocketPath: socketPath,
	}

	if err := StartWorker(worker, pythonBinPath, scriptPath, WorkerStartTimeout); err != nil {
		return nil, fmt.Errorf("worker.NewWorker: %w", err)
	}

	return worker, nil
}

// StartWorker is actually start the worker process and IPC for the worker and make those change to the worker
func StartWorker(w *types.Worker, pythonBinPath string, scriptPath string, startTimeout time.Duration) error {

	// taking FLCK on the worker
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	done := make(chan error, 1) // return channel

	errh := utils.ErrorHandler{
		Pkg: "worker",
		Fxn: "StartWorker",
	}

	// main function
	go func() {

		ls, err := ipc.CreateListener(w.SocketPath)
		if err != nil {
			done <- errh.Perror(err)
		}

		cmd, stdoutPipe, stderrPipe, err := SpawnWorkerProcess(pythonBinPath, scriptPath, w.SocketPath)
		if err != nil {
			done <- errh.Perror(err)
		}

		w.Cmd = cmd
		w.StdOutPipe = stdoutPipe
		w.StdErrPipe = stderrPipe

		conn, err := ipc.AcceptWorker(ls, nil)
		if err != nil {
			ls.Close()
			done <- errh.Perror(err)
		}

		if err := ipc.WaitForHelo(conn, nil); err != nil {
			done <- nil
		}

		ls.Close()

		w.Conn = conn

	}()

	select {
	case err := <-done:
		return err
	case <-time.After(startTimeout):
		if w.Conn != nil {
			w.Conn.Close()
		}
		if w.Cmd != nil {
			KillGroup(w.Cmd)
		}
		w.Status = types.WorkerDead
		return errh.Serror("Timer: timeout")
	}

}
