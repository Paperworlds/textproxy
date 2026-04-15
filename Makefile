BINARY   := textproxy
INSTALL  := $(HOME)/.local/bin/$(BINARY)
VERSION  := $(shell grep '^var Version' main.go | sed 's/.*"\(.*\)".*/\1/')
GITHASH  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
LDFLAGS  := -s -w -X main.Version=$(VERSION) -X main.GitHash=$(GITHASH)

.PHONY: build run install test bench lint clean

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

run: build
	./$(BINARY)

install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BINARY) $(INSTALL)
	codesign --sign - --force --preserve-metadata=entitlements,requirements,flags $(INSTALL)
	@echo "Installed to $(INSTALL)"

test:
	go test ./...

bench:
	go test -bench=. -benchtime=3s ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
