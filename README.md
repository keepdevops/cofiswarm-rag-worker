# cofiswarm-rag-worker

RAG background jobs (`jobs.py`) and auto-index queue (`auto_index.py` — no spawn).

The RAG store is serverless (sqlite-vec, a local `.db` file) — there is no
database container to provision.

- Worker daemon: `scripts/run-worker.py` (drains the FHS index queue, `/healthz` on `:8018`)
- Index state: `/var/lib/cofiswarm/rag/index`
