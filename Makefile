.PHONY: all test configsync run install deploy fmt vet generate local-ci

all: test configsync

# Run tests
test: generate fmt vet
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build configsync binary
configsync: generate fmt vet
	go build -o bin/configsync ./cmd/configsync

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/configsync/main.go

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

local-ci:
	gitlab-runner exec docker go-build \
			--env CI_COMMIT_REF_SLUG=$(shell git rev-parse --abbrev-ref HEAD) \
			--env CI_PROJECT_DIR=zedge \
			--env CI_PROJECT_PATH=zedge/config-sync-sidecar \
			--docker-pull-policy=if-not-present
