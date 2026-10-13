import json
import sys
from datetime import datetime as dt


class Logger:
    def __init__(self):
        self.taskId = None

    def set_taskId(self, taskId: str):
        self.taskId = taskId

    def log(self, message: str, module: str, internal: bool, critical: bool, type: str):
        payload = {
            "timestamp": str(dt.now()),
            "taskId": self.taskId,
            "internal": internal,
            "critical": critical,
            "module": module,
            "message": message,
        }
        if type == "error":
            sys.stderr.write(json.dumps(payload) + "\n")

        else:
            sys.stdout.write(json.dumps(payload) + "\n")
            sys.stdout.flush()

    def info(self, message: str, module: str, internal: bool = False):
        return self.log(message, module, internal, False, "info")

    def error(
        self, message: str, module: str, internal: bool = False, critical: bool = False
    ):
        return self.log(message, module, internal, critical, "error")
