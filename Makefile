BINARY  := textproxy
INSTALL := $(HOME)/.local/bin/$(BINARY)

.PHONY: build run install test bench lint clean

build:
	go build -ldflags="-s -w" -o $(BINARY) .

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
