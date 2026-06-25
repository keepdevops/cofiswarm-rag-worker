ROLE := rag-worker
.PHONY: build test test-standalone-layout test-gate
build:
	go build -o bin/cofiswarm-rag-worker ./cmd/cofiswarm-rag-worker
test: build test-standalone-layout test-gate
test-standalone-layout:
	./test/scripts/assert-layout.sh $(ROLE)
test-gate:
	./test/scripts/test-gate.sh
