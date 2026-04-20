BINARY   := textproxy
INSTALL  := $(HOME)/.local/bin/$(BINARY)
VERSION  := $(shell grep '^var Version' version.go | sed 's/.*"\(.*\)".*/\1/')
GITHASH  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
LDFLAGS  := -s -w -X main.Version=$(VERSION) -X main.GitHash=$(GITHASH)

.PHONY: build run install deploy test bench lint clean

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

run: build
	./$(BINARY)

# install copies the binary to ~/.local/bin but does NOT restart the daemon.
# Use 'make deploy' to also trigger a graceful restart.
install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BINARY) $(INSTALL)
	codesign --sign - --force --preserve-metadata=entitlements,requirements,flags $(INSTALL)
	@echo "Installed to $(INSTALL)"

# deploy installs then sends SIGUSR1 for a graceful in-place restart.
# Active streams are drained (up to 30s) before the new binary takes over.
deploy: install
	$(INSTALL) restart

test:
	go test ./...

bench:
	go test -bench=. -benchtime=3s ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
