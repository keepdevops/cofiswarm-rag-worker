package bus

import (
	"encoding/json"
	"testing"

	"github.com/keepdevops/cofiswarm-observer-sdk/pkg/servicecomponent"
)

func TestInfoRouteReturnsService(t *testing.T) {
	out, err := Routes()[servicecomponent.Prefix+".rag-worker.info"](nil)
	if err != nil {
		t.Fatal(err)
	}
	if r := out.(infoReply); !r.OK || r.Service != "rag-worker" {
		t.Fatalf("got %+v", r)
	}
}

func TestHealthRouteOK(t *testing.T) {
	out, _ := Routes()[servicecomponent.Prefix+".rag-worker.health"](nil)
	if r := out.(healthReply); !r.OK || r.Status != "ok" {
		t.Fatalf("got %+v", r)
	}
}

func TestReplyCarriesSchemaVersion(t *testing.T) {
	out, _ := Routes()[servicecomponent.Prefix+".rag-worker.info"](nil)
	b, _ := json.Marshal(out)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["schema_version"] != servicecomponent.SchemaVersion {
		t.Fatalf("schema_version = %v, want %s", m["schema_version"], servicecomponent.SchemaVersion)
	}
}
