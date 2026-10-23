# Go binaries of the BB-8. Static (CGO off), stripped.
#   make            # bin/ for this PC (bb8-body, bb8-head, bb8-web)
#   make pi         # dist/linux-arm64/bb8-body + bb8-head  (Pi 3 B+, 64-bit OS)
#   make zero       # dist/linux-armv6/bb8-head             (Pi Zero W, armv6)
#   make release    # pi + zero
#   make test       # go vet + go test
#   make probar     # dummy + Go binaries + end-to-end test (fresh dummy)
GO      ?= $(shell command -v go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
# Google sign-in client (GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET): gitignored, see google_client.env.example
-include google_client.env
CAL     := github.com/erickdsama/bb8/go/internal/connectors/calendar
FLAGS   := -trimpath -ldflags "-s -w -X $(CAL).builtinID=$(GOOGLE_CLIENT_ID) -X $(CAL).builtinSecret=$(GOOGLE_CLIENT_SECRET)"
PY      ?= $(shell [ -x .venv/bin/python ] && echo .venv/bin/python || echo python3)
export CGO_ENABLED=0

.PHONY: all pi zero release test probar clean
all:
	$(GO) build $(FLAGS) -o bin/ ./go/cmd/...

pi:
	GOOS=linux GOARCH=arm64 $(GO) build $(FLAGS) -o dist/linux-arm64/ ./go/cmd/...

zero:
	GOOS=linux GOARCH=arm GOARM=6 $(GO) build $(FLAGS) -o dist/linux-armv6/bb8-head ./go/cmd/bb8-head

release: pi zero
	@ls -la dist/*/

test:
	$(GO) vet ./go/...
	$(GO) test ./go/...

probar: all
	@$(PY) simulador/lanzar_dummy.py --camara sintetica --sin-sonido > /tmp/bb8-dummy.log 2>&1 & echo $$! > /tmp/bb8-dummy.pid
	@for i in $$(seq 60); do grep -q "dummy listo" /tmp/bb8-dummy.log && break; sleep 1; done
	@PYTHONPATH=simulador $(PY) -m dummy.probar; s=$$?; kill $$(cat /tmp/bb8-dummy.pid); exit $$s

clean:
	rm -rf bin dist
