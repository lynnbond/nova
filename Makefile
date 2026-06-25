# Nova Workflow Engine — Build Tool
.PHONY: build build-web build-all test clean run

# Build the frontend
build-web:
	@echo "→ Building frontend..."
	@cd web && npm install --silent && npm run build 2>&1 | tail -3

# Build the Go binary (standalone server)
build: build-web
	@echo "→ Building Go binary..."
	@PATH="/usr/local/go/bin:$$PATH" go build -o bin/nova-server ./cmd/nova-server/
	@echo "✓ bin/nova-server ready"

# Build the PDK library only (no frontend)
build-pdk:
	@go build ./...

# Build and run
run: build
	@echo "→ Starting Nova server on :8080"
	@./bin/nova-server --port 8080 --db nova.db

# Run tests
test:
	@go test -count=1 -timeout 30s ./...

# Run tests with verbose output
test-v:
	@go test -v -count=1 -timeout 30s ./...

# Clean build artifacts
clean:
	@rm -rf bin/ internal/app/webdist/ web/node_modules/ web/dist/

# Watch: rebuild frontend on changes (requires entr)
watch:
	@find web/src web/index.html -type f | entr -r make build-web
