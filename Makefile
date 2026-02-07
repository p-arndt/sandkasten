.PHONY: build runner images daemon clean

# Build everything
build: runner daemon images

# Build the runner binary (static, linux)
runner:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o images/base/runner ./cmd/runner

# Build the daemon binary
daemon:
	go build -o bin/sandkasten ./cmd/sandkasten

# Build Docker images
images: runner
	docker build -t sandbox-runtime:base ./images/base
	docker build -t sandbox-runtime:python ./images/python
	docker build -t sandbox-runtime:node ./images/node

# Build just base image (fastest iteration)
image-base: runner
	docker build -t sandbox-runtime:base ./images/base

# Run the daemon locally
run: daemon
	./bin/sandkasten

# Clean build artifacts
clean:
	rm -f images/base/runner bin/sandkasten sandkasten.db
