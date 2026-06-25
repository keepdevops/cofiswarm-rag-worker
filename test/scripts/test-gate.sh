#!/usr/bin/env bash
# Functional gate: the rag-worker boots, serves /healthz, and drains a queued job.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="${ROOT}/bin/cofiswarm-rag-worker"
PORT="${RAG_WORKER_TEST_PORT:-18018}"

TMP="$(mktemp -d)"
export COFISWARM_VAR_LIB="$TMP"
QDIR="${TMP}/rag/index/queue"
mkdir -p "$QDIR"
echo '{"path":"/src/kvrouter.go","status":"queued"}' > "${QDIR}/kvrouter.go.json"

"$BIN" -listen ":${PORT}" -poll 200ms &
PID=$!
trap 'kill $PID 2>/dev/null || true; rm -rf "$TMP"' EXIT
sleep 1

curl -s "http://127.0.0.1:${PORT}/healthz" | grep -q ok
python3 -c "import json; d=json.load(open('${QDIR}/kvrouter.go.json')); assert d['status']=='done', d; assert 'processed_at' in d; assert d['path']=='/src/kvrouter.go'"
echo "ok: rag-worker drains queue + /healthz"
