def generate_response(taskId, status, result, error = None):
    return {
        "taskId": taskId,
        "status": status,
        "result": result,
        "error": error
    }
