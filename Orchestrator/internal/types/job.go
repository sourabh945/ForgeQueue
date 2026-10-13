package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// JobStatus is where a job is in its life.
type JobStatus string

const (
	JobQueued  JobStatus = "queued"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
	JobDead    JobStatus = "dead"
)

type Job struct {
	TaskId      string          `json:"taskId"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Status      JobStatus       `json:"status"`
	Attempts    int             `json:"attempts"`    // times the orch has started this job
	MaxAttempts int             `json:"maxAttempts"` // maximum time orch can start a worker
	WebhookURL  string          `json:"webhookUrl"`  // url for POST the outcome
	CreatedAt   string          `json:"createdAt"`
	StartedAt   string          `json:"startedAt"`
	FinishedAt  string          `json:"finishedAt"`
	Result      json.RawMessage `json:"result"` // null until the job is done
	Error       *string         `json:"error"`  // null unless the job failed
}

// NowUTC returns the current tiem in the ISO format
// UTC, ISO-8601, trailing Z (for example 2026-10-12T01:01:00Z)
func NowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func atoi(field, value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	num, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("failed %s is not an integer: %w", field, err)
	}
	return num, nil
}

// JobFromHash builds a Job from the map returned by cache (redis)
func JobFromHash(h map[string]string, defaultMaxAttempts int) (*Job, error) {
	id := h["taskId"]
	if id == "" {
		return nil, errors.New("types.JobFromHash: missing taskId")
	}
	if h["type"] == "" {
		return nil, fmt.Errorf("types.JobFromHash: job %s has no type", id)
	}
	if !json.Valid([]byte(h["payload"])) {
		return nil, fmt.Errorf("types.JobFromHash: job %s has a misisng or invalid payload", id)
	}

	attempts, err := atoi("attempts", h["attempts"])
	if err != nil {
		return nil, fmt.Errorf("types.JobFromHash: job %s: %w", id, err)
	}

	maxAttempts, err := atoi("maxAttempts", h["maxAttempts"])
	if err != nil {
		return nil, fmt.Errorf("types.JobFromHash: job %s: %w", id, err)
	}
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}

	job := &Job{
		TaskId:      id,
		Type:        h["type"],
		Payload:     json.RawMessage(h["payload"]),
		Status:      JobStatus(h["status"]),
		Attempts:    attempts,
		MaxAttempts: maxAttempts,
		WebhookURL:  h["webhookUrl"],
		CreatedAt:   h["createdAt"],
		StartedAt:   h["startedAt"],
		FinishedAt:  h["finisedAt"],
	}

	if raw := h["result"]; raw != "" {
		if !json.Valid([]byte(raw)) {
			return nil, fmt.Errorf("types.JobFromHash: job %s has invalid result JSON", id)
		}
		job.Result = json.RawMessage(raw)
	}
	if msg := h["error"]; msg != "" {
		job.Error = &msg
	}
	return job, nil
}

// ToTask build the message that goes over the socket to the worker
func (job *Job) ToTask() Task {
	return Task{
		TaskId:    job.TaskId,
		Type:      job.Type,
		Payload:   job.Payload,
		CreatedAt: job.CreatedAt,
	}
}

// CanRetry reports whether the job still has attempts left
func (job *Job) CanRetry() bool {
	return job.Attempts < job.MaxAttempts
}

// IsTerminal report whether the job has reached an end state
func (job *Job) IsTerminal() bool {
	return job.Status == JobDone || job.Status == JobFailed || job.Status == JobDead
}
