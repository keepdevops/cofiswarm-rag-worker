package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnqueueThenDrain(t *testing.T) {
	t.Setenv("COFISWARM_VAR_LIB", t.TempDir())

	// Enqueue writes a queued job file under <var-lib>/rag/index/queue/.
	jf, err := Enqueue("/src/kvrouter.go")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if filepath.Dir(jf) != Dir() {
		t.Errorf("job file %s not under %s", jf, Dir())
	}

	// A second, already-done job must be left untouched.
	doneFile := filepath.Join(Dir(), "already.json")
	_ = os.WriteFile(doneFile, []byte(`{"path":"/x","status":"done","note":"keep"}`), 0o644)

	n, err := DrainOnce(func(string, ...any) {})
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if n != 1 {
		t.Errorf("drained %d, want 1 (only the queued job)", n)
	}

	// The queued job is now done + stamped, original path preserved.
	var j map[string]any
	raw, _ := os.ReadFile(jf)
	_ = json.Unmarshal(raw, &j)
	if j["status"] != "done" {
		t.Errorf("status=%v, want done", j["status"])
	}
	if j["path"] != "/src/kvrouter.go" {
		t.Errorf("path not preserved: %v", j["path"])
	}
	if _, ok := j["processed_at"].(float64); !ok {
		t.Errorf("processed_at not stamped: %v", j["processed_at"])
	}

	// The already-done job's extra field survived (no reprocessing).
	var d map[string]any
	raw, _ = os.ReadFile(doneFile)
	_ = json.Unmarshal(raw, &d)
	if d["note"] != "keep" || d["processed_at"] != nil {
		t.Errorf("already-done job was mutated: %v", d)
	}

	// Draining again processes nothing.
	if n, _ := DrainOnce(func(string, ...any) {}); n != 0 {
		t.Errorf("second drain processed %d, want 0", n)
	}
}

func TestIndexRootEnvOverride(t *testing.T) {
	t.Setenv("COFISWARM_VAR_LIB", "/custom/lib")
	if got := IndexRoot(); got != "/custom/lib/rag/index" {
		t.Errorf("IndexRoot=%s", got)
	}
}
