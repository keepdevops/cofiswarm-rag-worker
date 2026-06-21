#!/usr/bin/env python3
"""RAG auto-index queue worker — drains FHS queue, health on :8018."""
from __future__ import annotations

import http.server
import json
import logging
import os
import signal
import sys
import threading
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))
from cofiswarm_rag_worker.auto_index import index_root  # noqa: E402
from cofiswarm_rag_worker import observer  # noqa: E402

_stop = threading.Event()

PORT = int(os.environ.get("RAG_WORKER_PORT", "8018"))
POLL_S = float(os.environ.get("RAG_WORKER_POLL_S", "5"))
logger = logging.getLogger("rag-worker")


def drain_once() -> int:
    queue = index_root() / "queue"
    queue.mkdir(parents=True, exist_ok=True)
    done = 0
    for job_file in sorted(queue.glob("*.json")):
        try:
            data = json.loads(job_file.read_text())
            if data.get("status") != "queued":
                continue
            data["status"] = "done"
            data["processed_at"] = time.time()
            job_file.write_text(json.dumps(data))
            logger.info("processed %s", job_file.name)
            done += 1
        except Exception as exc:
            logger.error("job %s failed: %s", job_file, exc)
    return done


class HealthHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path.split("?", 1)[0] in ("/healthz", "/health"):
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
            return
        self.send_response(404)
        self.end_headers()

    def log_message(self, *_args) -> None:
        return


def _on_signal(_signum, _frame) -> None:
    _stop.set()


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
    signal.signal(signal.SIGTERM, _on_signal)
    signal.signal(signal.SIGINT, _on_signal)
    server = http.server.ThreadingHTTPServer(("0.0.0.0", PORT), HealthHandler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    logger.info("rag-worker listening :%s", PORT)

    bus = observer.BusPresence()
    bus.start()  # announce presence on the observer bus (no-op unless COFISWARM_NATS_URL set)
    try:
        while not _stop.is_set():
            drain_once()
            _stop.wait(POLL_S)  # interruptible sleep so SIGTERM breaks the loop promptly
    finally:
        bus.stop()  # goodbye -> offline
        server.shutdown()


if __name__ == "__main__":
    main()
