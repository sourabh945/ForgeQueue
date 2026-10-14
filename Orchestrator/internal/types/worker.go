package types

import (
	"io"
	"net"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// WorkerStatus is the state of one worker process
type WorkerStatus string

const (
	WorkerStarting WorkerStatus = "starting" // process spawned, HELO not recevied yet
	WorkerFree     WorkerStatus = "free"     // connected and waiting for a task
	WorkerBusy     WorkerStatus = "busy"     // a task is in flight
	WorkerDead     WorkerStatus = "dead"     // worker crashed, timeout, or stopped; need a restart
)

type Worker struct {
	Mutex        sync.RWMutex    // Mutex for thread safety
	ID           string          // Unqiue identifier for the worker process
	Status       WorkerStatus    // Status: running, free, failed, completed
	SocketPath   string          // Path of the unix domain socket
	Cmd          *exec.Cmd       // process executor pointer
	Conn         net.Conn        // ipc Unix Domain socket connection
	StdOutPipe   io.ReadCloser   // stdout pipe of worker process for logging
	StdErrPipe   io.ReadCloser   // stderr pipe of worker process for logging
	Exited       <-chan struct{} // closed when the worker process has exited (crash, kill, normal exit)
	Job          *Task           // Running task of the worker
	JobResponse  *TaskResult     // Result of the running task of the worker
	RestartCount int             // restart count of the worker
	StartedAt    time.Time       // when the current process was started
}

// SetStatus change the status under the write lock
func (w *Worker) SetStatus(status WorkerStatus) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()
	w.Status = status
}

// GetStatus read the status under the read lock
func (w *Worker) GetStatus() WorkerStatus {
	w.Mutex.RLock()
	defer w.Mutex.RUnlock()
	return w.Status
}

// BeginTask records the task in flight and marks the worker busy in one step
func (w *Worker) BeginTask(task *Task) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()
	w.Job = task
	w.JobResponse = nil
	w.Status = WorkerBusy
}

// EndTask clear the task in flight, stores the results (nil if there was none)
// and moves the worker to next: WorkerFree after a normal reply, WorkerDead
// after a crash, timeout, or bad response
func (w *Worker) EndTask(result *TaskResult, next WorkerStatus) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()
	w.Job = nil
	w.JobResponse = result
	w.Status = next
}

// TermGroup kill all the group process of the worker using SIGTERM
// Its for make sure all group of the process ( worker + worker child process ) are killed
func (w *Worker) TermGroup() {
	syscall.Kill(-w.Cmd.Process.Pid, syscall.SIGTERM)
}

// KillGroup kill all the group process of the worker using SIGKILL
// Its for make sure all group of the process ( worker + worker child process ) are killed
// It uses the SIGKILL so if the worker not allowed to terminate it will do using the OS inforcer
func (w *Worker) KillGroup() {
	syscall.Kill(-w.Cmd.Process.Pid, syscall.SIGKILL)
}

// Release clear the worker all pointers and remove there reference
// And the left work is done by GC
func (w *Worker) Release() {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	// Close the socket connection
	if w.Conn != nil {
		w.Conn.Close()
		w.Conn = nil
	}

	// Kill the process if still running
	if w.Cmd != nil && w.Cmd.Process != nil {
		w.KillGroup()
		w.Cmd.Wait()
		w.Cmd = nil
	}

	// Close pipes
	if w.StdOutPipe != nil {
		w.StdOutPipe.Close()
		w.StdOutPipe = nil
	}
	if w.StdErrPipe != nil {
		w.StdErrPipe.Close()
		w.StdErrPipe = nil
	}

	// Clear job references (lets GC collect the task data)
	w.Job = nil
	w.JobResponse = nil

	w.Status = WorkerDead
}

// WorkerSnapshot is a copy of a worker's state that is safe to hand to the
// health endpoint or to JSON encoding
type WorkerSnapshot struct {
	ID           string       `json:"id"`
	Status       WorkerStatus `json:"status"`
	RestartCount int          `json:"restartCount"`
	StartedAt    time.Time    `json:"startedAt"`
	CurrentTask  string       `json:"currentTask,omitempty"`
}

// Snapshot copies the fields other goroutine may read
func (w *Worker) Snapshot() WorkerSnapshot {
	w.Mutex.RLock()
	defer w.Mutex.RUnlock()
	snapshot := WorkerSnapshot{
		ID:           w.ID,
		Status:       w.Status,
		RestartCount: w.RestartCount,
		StartedAt:    w.StartedAt,
	}
	if w.Job != nil {
		snapshot.CurrentTask = w.Job.TaskId
	}
	return snapshot
}
