package types

import (
	"encoding/json"
	"fmt"
)

const (
	TaskDone = "done"
	TaskFail = "fail"
)

type Task struct {
	TaskId    string          `json:"taskId"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"createdAt"`
}

type TaskResult struct {
	TaskId *string         `json:"taskId"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
	Error  *string         `json:"error"`
}

// IsDone, this function validate, matches the taskId, then tells the task is fail or done
// but it will return false when there is any kind of error in validation, matches the taskId
func (res *TaskResult) IsDone(taskId string) (bool, error) {
	if !(res.Status == TaskDone || res.Status == TaskFail) {
		return false, fmt.Errorf("malformed task return")
	}
	if res.TaskId == nil {
		return false, fmt.Errorf("taskId is null")
	}

	if *res.TaskId != taskId {
		return false, fmt.Errorf("taskId is not match with given taskId")
	}

	if res.Status == TaskDone {
		return true, nil
	} else {
		return false, nil
	}
}

// GetError it will return any internal error is happens
// It is used because sometime there is internal error but not the actual err
func (res *TaskResult) AnyError() error {
	return fmt.Errorf("%v", *res.Error)
}
