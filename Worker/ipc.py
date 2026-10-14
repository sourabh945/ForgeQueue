MAX_PAYLOAD_SIZE_IN_MB = 16
MAX_PAYLOAD_SIZE = MAX_PAYLOAD_SIZE_IN_MB * 1024 * 1024



def read_frame(sock):
    """Read a frame from the socket."""
    length_bytes = _recv_exact(
        sock, 4
    )  # using 4 big-endint bytes for telling the size of the message
    if length_bytes == b"":
        return b""
    msg_length = int.from_bytes(length_bytes, "big")
    if msg_length > MAX_PAYLOAD_SIZE:
        raise Exception("Max Size Limit Exceeded")
    return _recv_exact(sock, msg_length)


def write_frame(sock, data: bytes):
    """Write to the socket"""
    length_prefix = len(data).to_bytes(4, "big")
    if len(data) > MAX_PAYLOAD_SIZE:
        raise Exception("Max Size Limit Exceeded")
    sock.sendall(length_prefix)
    sock.sendall(data)


def write_helo(sock):
    "for telling the orchestrator, worker is ready"
    message = b"HELO"
    write_frame(sock, message)


def _recv_exact(sock, n: int):
    buf = b""
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if chunk == b"":
            return b""  # peer closed connection in mid-read
        buf += chunk
    return buf
