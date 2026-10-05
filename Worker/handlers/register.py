HANDERS = {}

def register(task_type: str):
    """
        For register the handler functions to the system
    """
    def decorator(func):
        HANDERS[task_type] = func
        return func
    return decorator

def get_handler(task_type: str):
    """
        To get the handler function based upon the registery
    """
    return HANDERS.get(task_type)
