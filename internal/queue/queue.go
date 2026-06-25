// Package queue drains the RAG auto-index FHS queue and enqueues new jobs.
// Ports cofiswarm_rag_worker/auto_index.py (index_root/enqueue) + run-worker.py's
// drain_once: each queued job file under <var-lib>/rag/index/queue/*.json is flipped
// to done with a processed_at stamp; all other fields are preserved verbatim.
package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// IndexRoot returns <COFISWARM_VAR_LIB|/var/lib/cofiswarm>/rag/index (ports index_root()).
func IndexRoot() string {
	lib := os.Getenv("COFISWARM_VAR_LIB")
	if lib == "" {
		lib = "/var/lib/cofiswarm"
	}
	return filepath.Join(lib, "rag", "index")
}

// Dir is the directory holding pending job files.
func Dir() string { return filepath.Join(IndexRoot(), "queue") }

// DrainOnce processes every queued job file in the queue dir (status queued -> done),
// returning the count processed. Per-file failures are reported via logf (loud, never
// silent — CLAUDE.md §2) and do not abort the sweep. Unknown fields are preserved.
func DrainOnce(logf func(format string, args ...any)) (int, error) {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir queue %s: %w", dir, err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return 0, fmt.Errorf("glob queue %s: %w", dir, err)
	}
	sort.Strings(matches)
	done := 0
	for _, f := range matches {
		raw, err := os.ReadFile(f)
		if err != nil {
			logf("job %s read failed: %v", f, err)
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil {
			logf("job %s parse failed: %v", f, err)
			continue
		}
		if s, _ := data["status"].(string); s != "queued" {
			continue
		}
		data["status"] = "done"
		data["processed_at"] = float64(time.Now().UnixNano()) / 1e9 // time.time() seconds
		out, err := json.Marshal(data)
		if err != nil {
			logf("job %s marshal failed: %v", f, err)
			continue
		}
		if err := os.WriteFile(f, out, 0o644); err != nil {
			logf("job %s write failed: %v", f, err)
			continue
		}
		logf("processed %s", filepath.Base(f))
		done++
	}
	return done, nil
}

// Enqueue writes a queued job file for path (ports auto_index.enqueue), returning its path.
func Enqueue(path string) (string, error) {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir queue %s: %w", dir, err)
	}
	jobFile := filepath.Join(dir, filepath.Base(path)+".json")
	out, _ := json.Marshal(map[string]any{"path": path, "status": "queued"})
	if err := os.WriteFile(jobFile, out, 0o644); err != nil {
		return "", fmt.Errorf("write job %s: %w", jobFile, err)
	}
	return jobFile, nil
}
