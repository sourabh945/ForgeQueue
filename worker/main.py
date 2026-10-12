import json
import signal
import socket
import sys
import time

from global_logger import Logger

logger = Logger()

from handlers import get_handler

# Import the worker function from the worker module
from ipc import read_frame, write_frame, write_helo
from response_generator import generate_response as gr

SOCKET_PATH = ""
client = None

try:
    SOCKET_PATH = sys.argv[1]
except IndexError:
    logger.error(
        "Argument Error: <socket_path> is not given", "Main.Argument", True, True
    )
    sys.exit(1)


def graceful_shutdown(signum, frame):
    logger.info(
        f"Recieved singal {signum}, shutting down...",
        "Main.Signal Handler",
        True,
        False,
    )
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
sock = None
try:
    sock = connect_with_retry(SOCKET_PATH)
    logger.info(f"Connected to orchestrator at {SOCKET_PATH}", "Main", True)
except Exception as err:
    logger.error(str(err), "Main.Connection_with_retry", True, True)
    sys.exit(2)
# send the initial HELO message for telling orch the system is ready

write_helo(sock)
logger.info("Send HELO to the orch", "Main.write_helo", True)

# main loop - read task, processing , response, repeat
while True:
    taskId = None
    try:
        raw = read_frame(sock)
    except Exception as e:
        logger.error(f"read_frame failed: {e}", "Main.MainWorkerLoop", True, True)
        break

    if raw == b"":
        logger.error("orchestrator close connection", "Main.MainWorkerLoop", True, True)
        break

    try:
        task = json.loads(raw)

        task_type = task.get("type")
        handler = get_handler(task_type)
        taskId = task.get("taskId")

        # setting the taskId into the logger for bette logging
        logger.set_taskId(taskId)
        logger.info("Revcived the task", "Main.MainWorkerLoop", True)

        if handler is None:
            result = gr(
                taskId,
                "failed",
                {"outputFile": None, "error": "Unsupported task type"},
                None,
            )
            logger.error("Unsupported task type", "Main.MainWorkerLoop")
        else:
            try:
                output, status = handler(task.get("payload", {}))
                result = gr(
                    taskId,
                    status,
                    output,
                    None,
                )
            except FileNotFoundError:
                result = gr(
                    taskId,
                    "fail",
                    {"outputFile": None, "error": "Internal Error"},
                    "file is not found",
                )
                logger.error("FileNotFoundError", "Main.MainWorkerLoop", True, True)
            except Exception as e:
                result = gr(
                    taskId,
                    "failed",
                    {"outputFile": None, "error": str(e)},
                    None,
                )
                logger.error(str(e), "Main.MainWorkerLoop")
    except Exception as e:
        result = gr(
            None,
            "failed",
            {"outputFile": None, "error": "Internal Error"},
            f"malformed task: {e}",
        )
        logger.error("Malformed task", "Main.MainWorkerLoop", True, True)
    try:
        write_frame(sock, json.dumps(result).encode("utf-8"))
        logger.info("Task Completed Successfully", "Main.MainWorkerLoop")
    except Exception as e:
        logger.error(f"write_frame failed : {e} ", "Main.MainWorkerLoop", True)
        break

sock.close()
