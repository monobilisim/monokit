.PHONY: help all with-api clean clean-coverage clean-plugins docs install install-with-api test test-with-api build-plugins build-plugin-k8sHealth build-plugin-redisHealth install-deps .FORCE

# Colors for help target
BLUE := \033[34m
CYAN := \033[36m
GREEN := \033[32m
RED := \033[31m
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
	@echo "  $(GREEN)make build-plugins$(RESET)     Build plugins for current platform"
	@echo "  $(GREEN)make gen-health-plugin-proto$(RESET) Generate protobuf code for health plugins"
	@echo "  $(GREEN)make install-deps$(RESET)       Install Go dependencies for protobuf generation"
	@echo "  $(GREEN)make clean$(RESET)              Clean all build artifacts"
	@echo "  $(GREEN)make clean-plugins$(RESET)      Clean only plugin build artifacts"
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
	go test -tags=with_api -coverprofile=coverage.out -coverpkg=github.com/monobilisim/monokit/common/api/... ./common/api/tests
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Tests complete, coverage report written to coverage.html$(RESET)"

# Clean coverage files
clean-coverage:
	@echo "$(BLUE)Cleaning coverage files...$(RESET)"
	rm -f coverage.out coverage.html *.coverprofile
	@echo "$(GREEN)Coverage files cleaned$(RESET)"

# Build with API support
with-api: bin/monokit-with-api

# Clean all build artifacts
clean: clean-coverage clean-plugins
	@echo "$(BLUE)Cleaning all build artifacts...$(RESET)"
	rm -rf bin/
	@echo "$(GREEN)Clean complete$(RESET)"

# Clean plugins
clean-plugins:
	@echo "$(BLUE)Cleaning plugin artifacts...$(RESET)"
	rm -rf $(PLUGINS_DIR)/*
	@echo "$(GREEN)Plugin clean complete$(RESET)"

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

# --- Plugin Proto generation (health_plugin.proto) ---
PROTO_DIR=proto
HEALTH_PROTO=$(PROTO_DIR)/health_plugin.proto
HEALTH_PROTO_GO_PKG=common/health/pluginpb

install-deps:
	@echo "$(BLUE)Installing Go dependencies for protobuf generation...$(RESET)"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "$(GREEN)Dependencies installed$(RESET)"

gen-health-plugin-proto: .FORCE
	@echo "$(BLUE)Generating health plugin protobuf code...$(RESET)"
	@mkdir -p $(HEALTH_PROTO_GO_PKG)
	@if ! command -v protoc-gen-go >/dev/null 2>&1; then \
		echo "$(RED)Error: protoc-gen-go not found$(RESET)"; \
		echo "$(BLUE)Please run: make install-deps$(RESET)"; \
		exit 1; \
	fi
	@if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then \
		echo "$(RED)Error: protoc-gen-go-grpc not found$(RESET)"; \
		echo "$(BLUE)Please run: make install-deps$(RESET)"; \
		exit 1; \
	fi
	protoc --go_out=paths=source_relative:$(HEALTH_PROTO_GO_PKG) \
	       --go-grpc_out=paths=source_relative:$(HEALTH_PROTO_GO_PKG) \
	       $(HEALTH_PROTO)
	@echo "$(GREEN)Health plugin protobuf code generated$(RESET)"

.PHONY: gen-health-plugin-proto

# --- Plugin Building ---
PLUGINS_DIR=plugins

build-plugins: gen-health-plugin-proto build-plugin-k8sHealth build-plugin-redisHealth build-plugin-wppconnectHealth
	@echo "$(GREEN)All plugins built.$(RESET)"

build-plugin-k8sHealth: gen-health-plugin-proto .FORCE
	@echo "$(BLUE)Building k8sHealth plugin for current platform...$(RESET)"
	@mkdir -p $(PLUGINS_DIR)
	
	# Build for current platform
	cd k8sHealth && go build -tags=plugin -o ../$(PLUGINS_DIR)/k8sHealth ./cmd/plugin/main.go
	
	@echo "$(GREEN)k8sHealth plugin built: $(PLUGINS_DIR)/k8sHealth$(RESET)"

build-plugin-redisHealth: gen-health-plugin-proto .FORCE
	@echo "$(BLUE)Building redisHealth plugin for current platform...$(RESET)"
	@mkdir -p $(PLUGINS_DIR)
	
	# Build for current platform
	cd redisHealth && go build -tags=plugin -o ../$(PLUGINS_DIR)/redisHealth ./cmd/plugin/main.go
	
	@echo "$(GREEN)redisHealth plugin built: $(PLUGINS_DIR)/redisHealth$(RESET)"

build-plugin-wppconnectHealth: gen-health-plugin-proto .FORCE
	@echo "$(BLUE)Building wppconnectHealth plugin for current platform...$(RESET)"
	@mkdir -p $(PLUGINS_DIR)
	
	# Build for current platform
	cd wppconnectHealth && go build -tags=plugin -o ../$(PLUGINS_DIR)/wppconnectHealth ./cmd/plugin/main.go
	
	@echo "$(GREEN)wppconnectHealth plugin built: $(PLUGINS_DIR)/wppconnectHealth$(RESET)"

build-plugin-pritunlHealth: gen-health-plugin-proto .FORCE
	@echo "$(BLUE)Building pritunlHealth plugin for current platform...$(RESET)"
	@mkdir -p $(PLUGINS_DIR)
	
	# Build for current platform
	cd pritunlHealth && go build -tags=plugin -o ../$(PLUGINS_DIR)/pritunlHealth ./cmd/plugin/main.go
	
	@echo "$(GREEN)pritunlHealth plugin built: $(PLUGINS_DIR)/pritunlHealth$(RESET)"