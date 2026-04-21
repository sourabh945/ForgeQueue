import socket
import os
import json
import sys
import signal

# Import the worker *function* from the worker *module*
from worker_func import worker

# Initialize global variables so signal handlers don't crash if called early
SOCKET_PATH = ""
server = None

# Reading the socket file path from argv
try:
    SOCKET_PATH = sys.argv[1]
except IndexError:
    print("Usage: python main.py <socket_path>")
    sys.exit(1)

# ---------------------- Helper functions ------------------------------------------------------

def clean_socketFile(file_path: str) -> None:
    """
    Find the socket file and, if it is found, remove it.
    """
    if os.path.exists(file_path):
        os.remove(file_path)

# ---------------------- Handling SIGs for graceful shutdown -----------------------------------

def graceful_shutdown(signum, frame):
    """
    This function will be called when a signal is received.
    """
    print(f"\nReceived signal {signum}, shutting down...")

    # Safely close the server if it was initialized
    if server:
        server.close()

    clean_socketFile(SOCKET_PATH)
    sys.exit(0)

signal.signal(signal.SIGINT, graceful_shutdown)
signal.signal(signal.SIGTERM, graceful_shutdown)

# ----------------------- Main logic -----------------------------------------------------------

# Clean up the socket file if it already exists from a previous crash
clean_socketFile(SOCKET_PATH)

# Create a Unix Domain Socket (AF_UNIX)
try:
    server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    server.bind(SOCKET_PATH)
    server.listen(1)
    print(f"Worker listening on {SOCKET_PATH}...")
except Exception as e:
    print(f"Error: {e}\nUnable to open the socket connection for IPC")
    clean_socketFile(SOCKET_PATH)
    sys.exit(1)

# Main worker loop
while True:
    try:
        conn, addr = server.accept()
        # Use a context manager to ensure the connection is always closed
        with conn:
            # Receive data from Go Manager
            data = conn.recv(1024)
            if data:
                message = json.loads(data.decode('utf-8'))

                # Execute the worker function
                response = worker(message['JobId'], message['ImagePath'], message['Type'])

                # Send the response back
                conn.send(json.dumps(response).encode('utf-8'))

    except Exception as e:
        # Catching exceptions inside the loop ensures the server stays alive
        # even if a single request is malformed (e.g., bad JSON)
        print(f"Error: {e}\nUnable to process the request")
