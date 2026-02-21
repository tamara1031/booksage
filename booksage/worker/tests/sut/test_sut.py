import os
import signal
import socket
import subprocess
import time


def is_port_open(host, port):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        return s.connect_ex((host, port)) == 0


def test_worker_startup():
    # 1. Start the worker process
    # Assuming we are in worker/ directory when running pytest
    cmd = ["python", "src/booksage/main.py"]
    env = os.environ.copy()
    env["PYTHONPATH"] = "src"

    process = subprocess.Popen(cmd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    try:
        # 2. Give it some time to start
        start_time = time.time()
        timeout = 10
        success = False

        while time.time() - start_time < timeout:
            if is_port_open("localhost", 50051):
                success = True
                break
            time.sleep(0.5)

        assert success, "Worker failed to listen on port 50051 within timeout"

    finally:
        # 3. Shutdown
        process.send_signal(signal.SIGTERM)
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
