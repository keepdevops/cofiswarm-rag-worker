# cofiswarm-rag-worker

RAG auto-index queue worker (Go). Drains the FHS index queue, serves `/healthz`
on `:8018`, and optionally announces presence on the observer bus.

The RAG store is serverless (sqlite-vec, a local `.db` file) — there is no
database container to provision.

- **Daemon:** `cmd/cofiswarm-rag-worker` — `go build -o bin/cofiswarm-rag-worker ./cmd/cofiswarm-rag-worker`
- **Queue:** `internal/queue` drains `<COFISWARM_VAR_LIB|/var/lib/cofiswarm>/rag/index/queue/*.json` (status `queued` → `done` + `processed_at`); `Enqueue` adds jobs.
- **Bus presence:** `internal/bus` serves `.rag-worker.{info,health}` via cofiswarm-observer-sdk — default-off, enabled when `COFISWARM_NATS_URL` is set, graceful goodbye on SIGTERM.
- **Flags/env:** `-listen :8018` (`RAG_WORKER_PORT`), `-poll 5s` (`RAG_WORKER_POLL_S`).

Go port of the former Python daemon (`scripts/run-worker.py` + `auto_index.py` + `observer.py`).

## Test

```bash
make test   # build + standalone-layout + drain/healthz gate
```
