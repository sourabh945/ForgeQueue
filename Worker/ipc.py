import json


def read_frame(sock):
    """Read a frame from the socket."""
    length_bytes = _recv_exact(
        sock, 4
    )  # using 4 big-endint bytes for telling the size of the message
    if length_bytes == b"":
        return b""
    msg_length = int.from_bytes(length_bytes, "big")
    return _recv_exact(sock, msg_length)


def write_frame(sock, data: bytes):
    """Write to the socket"""
    length_prefix = len(data).to_bytes(4, "big")
    sock.sendall(length_prefix)
    sock.sendall(data)


def write_helo(sock):
    "for telling the orchestrator, worker is ready"
    message = b'HELO'
    write_frame(sock, message)


def _recv_exact(sock, n: int):
    buf = b""
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if chunk == b"":
            return b""  # peer closed connection in mid-read
        buf += chunk
    return buf
