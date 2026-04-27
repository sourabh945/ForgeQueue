package main

import (
	"encoding/json"
	"fmt"
	"net"
)

// Define the structure for our Job
type Job struct {
	JobId     string `json:"JobId"`
	ImagePath string `json:"ImagePath"`
	Type      string `json:"Type"`
}

func main() {
	socketPath := "/tmp/worker-1.sock"

	// Connect to the Unix Socket
	conn, err := net.Dial("unix", socketPath)

	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	testJob := Job{
		JobId:     "abc123",
		ImagePath: "/shared/input/test.jpg",
		Type:      "sharpen",
	}

	jsonData, _ := json.Marshal(testJob)

	fmt.Println("Sending job to Python...")

	conn.Write(jsonData)

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)

	if err != nil {
		fmt.Printf("Error reading response: %v\n ", err)
	}

	fmt.Printf("Received response: %s\n", string(buffer[:n]))

}
