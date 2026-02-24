.DEFAULT_GOAL := help

help:
	@echo "Usage: make <target> [ARGS=\"...\"]"
	@echo ""
	@echo "Build & Dev:"
	@echo "  build       Build the passforge binary"
	@echo "  test        Run all tests with verbose output"
	@echo "  bench       Run benchmarks with memory stats"
	@echo "  vet         Run go vet on all packages"
	@echo "  fmt         Format all Go source files"
	@echo "  cover       Run tests with coverage report"
	@echo "  all         Run vet + test + bench"
	@echo "  clean       Remove build artifacts"
	@echo ""
	@echo "Run Commands:"
	@echo "  generate    make generate ARGS=\"--length 20\""
	@echo "  passphrase  make passphrase ARGS=\"--words 4\""
	@echo "  check       make check ARGS=\"MyP@ssw0rd\""
	@echo "  suggest     make suggest ARGS=\"hello123\""
	@echo "  rotate      make rotate ARGS=\"p@sSwor4 --count 5\""
	@echo "  ssbd        make ssbd ARGS=\"p@sSwor4 --count 5\""
	@echo "  bulk        make bulk ARGS=\"--count 10 --length 16\""

# Build
build:
	go build -o passforge ./cmd/passforge

# --- Run Commands ---

generate:
	go run ./cmd/passforge generate $(ARGS)

passphrase:
	go run ./cmd/passforge passphrase $(ARGS)

check:
	go run ./cmd/passforge check $(ARGS)

suggest:
	go run ./cmd/passforge suggest $(ARGS)

rotate:
	go run ./cmd/passforge rotate $(ARGS)

ssbd:
	go run ./cmd/passforge ssbd $(ARGS)

bulk:
	go run ./cmd/passforge bulk $(ARGS)

# --- Dev ---

# Test
test:
	go test -v ./...

# Benchmarks
bench:
	go test -bench=. -benchmem ./...

# Lint
vet:
	go vet ./...

# Format
fmt:
	go fmt ./...

# Coverage
cover:
	go test -cover ./...

# All checks
all: vet test bench

# Clean
clean:
	rm -f passforge

.PHONY: help build generate passphrase check suggest rotate ssbd bulk test bench vet fmt cover all clean
