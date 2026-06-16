# cofiswarm-rag-worker

RAG background jobs (`jobs.py`) and auto-index queue (`auto_index.py` — no spawn).

- `scripts/rag-docker-compose.sh` → cofiswarm-launcher compose pgvector
- Index state: `/var/lib/cofiswarm/rag/index`
