package types

import "encoding/json"

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
