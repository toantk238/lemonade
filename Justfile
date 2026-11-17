set shell := ["bash", "-euo", "pipefail", "-c"]

version := `git describe --tags`
pkgs := ". ./client/... ./lemon/... ./param/... ./server/..."
ldflags := "-s -w -X github.com/lemonade-command/lemonade/lemon.Version=" + version
bin := "lemonade"
install_path := "/usr/bin/lemonade"
service := "lemonade-server"

# List recipes
default:
    @just --list

# Build ./lemonade
build:
    go build -ldflags "{{ldflags}}" -o {{bin}}

# Format, vet and test
check:
    test -z "$(gofmt -l client lemon param server main.go)"
    go vet {{pkgs}}
    go test {{pkgs}}

# Format Go sources
fmt:
    gofmt -w client lemon param server main.go

# Build, install to /usr/bin (sudo) and restart the user service
install: build
    sudo install -m 755 {{bin}} {{install_path}}
    systemctl --user restart {{service}}

# Restart the lemonade user service
restart:
    systemctl --user restart {{service}}

# Show the lemonade user service status
status:
    systemctl --user status {{service}} --no-pager

# Follow the lemonade user service logs
logs:
    journalctl --user -u {{service}} -f

# Run the freshly built server in the foreground on a test port
dev-server port="4321": build
    ./{{bin}} server --port={{port}} --log-level=1

# Copy a file through a dev server on the given port and show clipboard types
test-copy text_file port="4321":
    ./{{bin}} copy --host=127.0.0.1 --port={{port}} --trim-newline < "{{text_file}}"
    sleep 0.3
    wl-paste --list-types

# Remove build outputs
clean:
    rm -f {{bin}}
    make clean
