.PHONY: help all with-api clean clean-coverage docs build-frontend clean-frontend install install-with-api test test-with-api .FORCE

# Colors for help target
BLUE := \033[34m
CYAN := \033[36m
GREEN := \033[32m
RESET := \033[0m

# Go source files
GO_FILES := $(shell find . -type f -name '*.go')

# Force target to ensure rebuilding
.FORCE:

# Default target: build without API
all: bin/monokit

# Help target
help:
	@echo "$(CYAN)Monokit Build System$(RESET)"
	@echo "$(BLUE)Available targets:$(RESET)"
	@echo "  $(GREEN)make$(RESET)                    Build monokit (minimal build, no API)"
	@echo "  $(GREEN)make help$(RESET)               Show this help message"
	@echo "  $(GREEN)make with-api$(RESET)          Build monokit with API server (includes frontend)"
	@echo "  $(GREEN)make clean$(RESET)              Clean all build artifacts"
	@echo "  $(GREEN)make clean-frontend$(RESET)     Clean only frontend build artifacts"
	@echo "  $(GREEN)make clean-coverage$(RESET)     Clean coverage files"
	@echo "  $(GREEN)make docs$(RESET)               Generate swagger documentation"
	@echo "  $(GREEN)make install$(RESET)            Install minimal monokit"
	@echo "  $(GREEN)make install-with-api$(RESET)   Install monokit with API (includes frontend)"
	@echo "  $(GREEN)make test$(RESET)               Run tests"
	@echo "  $(GREEN)make test-with-api$(RESET)      Run tests including API"

# Run tests without API
test:
	@echo "$(BLUE)Running tests...$(RESET)"
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Tests complete, coverage report written to coverage.html$(RESET)"

# Run tests with API support
test-with-api:
	@echo "$(BLUE)Running tests with API...$(RESET)"
	go test -tags=with_api -coverprofile=coverage.out -coverpkg=github.com/monobilisim/monokit/common/api ./common/api/tests
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Tests complete, coverage report written to coverage.html$(RESET)"

# Clean coverage files
clean-coverage:
	@echo "$(BLUE)Cleaning coverage files...$(RESET)"
	rm -f coverage.out coverage.html *.coverprofile
	@echo "$(GREEN)Coverage files cleaned$(RESET)"

# Build with API support (includes frontend)
with-api: clean-frontend build-frontend bin/monokit-with-api

# Build the frontend assets
build-frontend:
	@echo "$(BLUE)Building frontend assets...$(RESET)"
	cd frontend && npm ci && npm run build
	mkdir -p common/api/frontend/build
	cp -r frontend/build/* common/api/frontend/build/
	@echo "$(GREEN)Frontend build complete$(RESET)"

# Clean frontend build artifacts
clean-frontend:
	@echo "$(BLUE)Cleaning frontend artifacts...$(RESET)"
	rm -rf frontend/build
	rm -rf common/api/frontend/build
	@echo "$(GREEN)Frontend clean complete$(RESET)"

# Clean all build artifacts
clean: clean-frontend clean-coverage
	@echo "$(BLUE)Cleaning all build artifacts...$(RESET)"
	rm -rf bin/
	@echo "$(GREEN)Clean complete$(RESET)"

# Generate swagger documentation
docs:
	@echo "$(BLUE)Generating swagger documentation...$(RESET)"
	swag init --parseDependency --parseInternal -g common/api/server.go
	@echo "$(GREEN)Documentation generation complete$(RESET)"

# Build minimal binary (no API)
bin/monokit: .FORCE
	@echo "$(BLUE)Building minimal monokit...$(RESET)"
	@mkdir -p bin
	@rm -f bin/monokit
	@rm -rf common/api/frontend/build
	CGO_ENABLED=0 go build -tags "" -o bin/monokit
	# Only strip when not cross-compiling
	if [ "$(shell go env GOOS)" = "$(shell go env GOHOSTOS)" ] && [ "$(shell go env GOARCH)" = "$(shell go env GOHOSTARCH)" ]; then \
		strip bin/monokit || true; \
	fi
	@echo "$(GREEN)Build complete: bin/monokit$(RESET)"

# Build binary with API (includes frontend)
bin/monokit-with-api: .FORCE
	@echo "$(BLUE)Building monokit with API...$(RESET)"
	@mkdir -p bin
	@rm -f bin/monokit
	CGO_ENABLED=0 go build -tags "with_api" -o bin/monokit
	# Only strip when not cross-compiling
	if [ "$(shell go env GOOS)" = "$(shell go env GOHOSTOS)" ] && [ "$(shell go env GOARCH)" = "$(shell go env GOHOSTARCH)" ]; then \
		strip bin/monokit || true; \
	fi
	@echo "$(GREEN)Build complete: bin/monokit (with API)$(RESET)"

# Install minimal build
install: bin/monokit
	@echo "$(BLUE)Installing minimal monokit...$(RESET)"
	install -m 755 bin/monokit /usr/local/bin/monokit
	@echo "$(GREEN)Installation complete$(RESET)"

# Install with API
install-with-api: with-api
	@echo "$(BLUE)Installing monokit with API...$(RESET)"
	install -m 755 bin/monokit /usr/local/bin/monokit
	@echo "$(GREEN)Installation complete$(RESET)"
