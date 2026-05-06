package executor

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	ipc "github.com/sourabh945/ForgeQueue/Orchestrator/internal/ipc"
	Types "github.com/sourabh945/ForgeQueue/Orchestrator/internal/types"
)

func InitWorkerProcess(cmd []string, _logger *slog.Logger, workerCount int) Worker {

	// making the socket path and worker ID
	socketPath := fmt.Sprintf("/tmp/forgequeue-worker-%d.sock", workerCount)

	id := fmt.Sprintf("worker-%d", workerCount)

	// creating the module logger and worker-specific logger
	moduleLogger := _logger.With(slog.String("type", "orch.module"), slog.String("module", "InitWorkerProcess"))

	logger := moduleLogger.With(slog.String("id", id))

	// removing any existing socket file (cleanup)
	os.Remove(socketPath)

	// creating the command to execute
	_cmd := append(cmd, socketPath)

	// executing the command
	command := exec.Command(_cmd[0], _cmd[1:]...)

	// setting the process group ID to the worker's PID
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// getting the stdout and stderr pipes
	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		moduleLogger.Error("failed to create stdout pipe", "error", err)
		return Worker{}
	}

	stderrPipe, err := command.StderrPipe()
	if err != nil {
		moduleLogger.Error("failed to create stderr pipe", "error", err)
		return Worker{}
	}

	//start the process
	if err := command.Start(); err != nil {
		moduleLogger.Error("failed to start process", "error", err)
		return Worker{}
	}

	// create the worker struct
	worker := Worker{
		Worker: &Types.Worker{
			ID:         id,
			Status:     "free",
			SocketPath: socketPath,
			Cmd:        command,
			Logger:     logger,
			StdOut:     stdoutPipe,
			StdErr:     stderrPipe,
			ExitCode:   256,
		},
	}

	// fire the goroutine to read from the stdout and stderr pipes
	go worker.StdOutLogger()
	go worker.StdErrLogger()

	// fire the goroutine to wait for the process to exit
	go worker.WaitForProcess()

	// fire the ipc connection
	conn := ipc.InitConnection(socketPath, logger)
	if conn == nil {
		worker.KillProcess(-2)
		moduleLogger.Error("failed to connect to socket", slog.Any("error", err))
		return Worker{}
	}

	worker.Conn = conn

	return worker
}
