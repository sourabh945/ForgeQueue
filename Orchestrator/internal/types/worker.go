package types

import (
	"io"
	"net"
	"os/exec"
	"sync"
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
	Mutex        sync.RWMutex  // Mutex for thread safety
	ID           string        // Unqiue identifier for the worker process
	Status       WorkerStatus  // Status: running, free, failed, completed
	SocketPath   string        // Path of the unix domain socket
	Cmd          *exec.Cmd     // process executor pointer
	Conn         net.Conn      // ipc Unix Domain socket connection
	StdOutPipe   io.ReadCloser // stdout pipe of worker process for logging
	StdErrPipe   io.ReadCloser // stderr pipe of worker process for logging
	Job          *Task         // Running task of the worker
	JobResponse  *TaskResult   // Result of the running task of the worker
	RestartCount int           // restart count of the worker
	StartedAt    time.Time     // when the current process was started
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
