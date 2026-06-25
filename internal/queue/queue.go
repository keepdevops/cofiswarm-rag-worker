// Package queue drains the RAG auto-index FHS queue and enqueues new jobs.
// Ports cofiswarm_rag_worker/auto_index.py (index_root/enqueue) + run-worker.py's
// drain_once: each queued job file under <var-lib>/rag/index/queue/*.json is flipped
// to done with a processed_at stamp; all other fields are preserved verbatim.
package queue

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// DrainOnce processes every queued job file (status queued -> done), returning the count
// processed. If process is non-nil it is called with the job's "path" (e.g. to ingest the file
// into the rag service); a process error leaves the job queued for retry and is logged. When
// process is nil the job is just marked done (the legacy flip-only behavior). Per-file failures
// are reported via logf (loud, never silent — CLAUDE.md §2) and do not abort the sweep.
func DrainOnce(process func(path string) error, logf func(format string, args ...any)) (int, error) {
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
		if process != nil {
			path, _ := data["path"].(string)
			if err := process(path); err != nil {
				logf("ingest %s failed: %v (left queued for retry)", path, err)
				continue // leave queued; next sweep retries
			}
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

// ScanAndEnqueue walks dir and enqueues any file with a matching extension that is new or has
// changed since it was last indexed (the queue dir is the seen-state: a file is (re)enqueued
// when it has no done job, or its mtime is newer than that job's processed_at). Returns the
// number enqueued. Used by the worker's poll loop for filesystem auto-indexing.
func ScanAndEnqueue(dir string, exts []string, logf func(format string, args ...any)) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !hasExt(p, exts) {
			return nil
		}
		fi, e := d.Info()
		if e != nil {
			logf("scan stat %s: %v", p, e)
			return nil
		}
		if needsIndex(p, fi.ModTime()) {
			if _, e := Enqueue(p); e != nil {
				logf("enqueue %s: %v", p, e)
			} else {
				n++
			}
		}
		return nil
	})
	return n, err
}

// needsIndex reports whether path should be (re)enqueued given its mtime, using the existing
// job file as state: no job => never indexed; already queued => skip; done & mtime newer than
// processed_at => changed since last index.
func needsIndex(path string, mtime time.Time) bool {
	raw, err := os.ReadFile(filepath.Join(Dir(), filepath.Base(path)+".json"))
	if err != nil {
		return true
	}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return true
	}
	if s, _ := data["status"].(string); s == "queued" {
		return false
	}
	pa, _ := data["processed_at"].(float64)
	return mtime.Unix() > int64(pa)
}

func hasExt(path string, exts []string) bool {
	lp := strings.ToLower(path)
	for _, e := range exts {
		if strings.HasSuffix(lp, e) {
			return true
		}
	}
	return false
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
