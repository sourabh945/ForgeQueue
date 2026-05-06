package manager

import (
	"time"

	executor "github.com/sourabh945/ForgeQueue/Orchestrator/internal/executor"
)

// WatchDog watches the worker and kills it if the job takes too long.
// done channel should be closed by the caller when the job completes.
func WatchDog(worker *executor.Worker, timeout time.Duration, done <-chan struct{}) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		// timeout hit — kill worker
		worker.KillProcess(-3)
	case <-done:
		// job completed in time — do nothing
	}
}
