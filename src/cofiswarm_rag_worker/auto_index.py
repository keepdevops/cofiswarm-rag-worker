"""Background index queue — no posix_spawn (Sprint 10). Enqueue paths for rag-worker."""
from __future__ import annotations

import json
import logging
import os
from pathlib import Path

logger = logging.getLogger(__name__)


def index_root() -> Path:
    lib = os.environ.get("COFISWARM_VAR_LIB", "/var/lib/cofiswarm")
    return Path(lib) / "rag" / "index"


def enqueue(path: Path, *, state_dir: Path | None = None) -> Path:
    state_dir = state_dir or (index_root() / "queue")
    state_dir.mkdir(parents=True, exist_ok=True)
    job_file = state_dir / f"{path.name}.json"
    job_file.write_text(json.dumps({"path": str(path), "status": "queued"}))
    logger.info("queued auto-index job %s", job_file)
    return job_file
