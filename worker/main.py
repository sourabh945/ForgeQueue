import json
import signal
import socket
import sys
import time

from handlers import get_handler

# Import the worker function from the worker module
from ipc import read_frame, write_frame, write_helo

SOCKET_PATH = ""
client = None

try:
    SOCKET_PATH = sys.argv[1]
except IndexError:
    print("Usage: python main.py <socket_path>")
    sys.exit(1)


def graceful_shutdown(signum, frame):
    print(f"\n Recieved singal {signum}, shutting down...")
    if client:
        client.close()
    sys.exit(0)


signal.signal(signal.SIGINT, graceful_shutdown)
signal.signal(signal.SIGTERM, graceful_shutdown)


def connect_with_retry(path, attempts=10, delay=0.1):
    # there is no need for retry mainly because socket is going to be open when we doing this but Justin Case
    for i in range(attempts):
        try:
            sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            sock.connect(path)
            return sock
        except (FileNotFoundError, ConnectionRefusedError):
            time.sleep(delay * i)
    raise RuntimeError(f"Could not connect to {path} after {attempts} attempts")


# unix domain sockets client for connection
sock = connect_with_retry(SOCKET_PATH)
print(f"Connected to orchestrator at {SOCKET_PATH}")

# send the initial HELO message for telling orch the system is ready
write_helo(sock)

# main loop - read task, processing , response, repeat
while True:
    taskId = None
    try:
        raw = read_frame(sock)
    except Exception as e:
        print(f"read_frame failed: {e}")
        break

    if raw == b"":
        print("orchestrator close connection")
        break

    try:
        task = json.loads(raw)
        task_type = task.get("type")
        handler = get_handler(task_type)
        taskId = task.get("taskId")
        if handler is None:
            result = {
                "taskId": taskId,
                "status": "failed",
                "result": None,
                "error": f"unsupported task type: {task_type}",
            }
        else:
            try:
                output, status = handler(task.get("payload", {}))
                result = {
                    "taskId": taskId,
                    "status": status,
                    "result": output,
                    "error": None,
                }
            except Exception as e:
                result = {
                    "taskId": taskId,
                    "status": "failed",
                    "result": None,
                    "error": str(e),
                }
    except Exception as e:
        result = {
            "taskId": None,
            "status": "failed",
            "result": None,
            "error": f"malformed task: {e}",
        }
        print("malformed task")

    try:
        write_frame(sock, json.dumps(result).encode("utf-8"))
    except Exception as e:
        print(f"write_frame failed : {e} ")
        break

sock.close()
