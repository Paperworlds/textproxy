binary := "textproxy"
install := home_dir() / ".local/bin" / binary

build:
    go build -ldflags="-s -w" -o {{binary}} .

run: build
    ./{{binary}}

install: build
    mkdir -p {{home_dir()}}/.local/bin
    cp {{binary}} {{install}}
    codesign --sign - --force --preserve-metadata=entitlements,requirements,flags {{install}}
    @echo "Installed to {{install}}"

test:
    go test ./...

bench:
    go test -bench=. -benchtime=3s ./...

lint:
    go vet ./...

clean:
    rm -f {{binary}}
