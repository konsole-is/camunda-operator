# Build the fake plugin of the kind harness of the fleet manager. The CI job
# of the fleet manager journeys loads this image into its kind cluster.
FROM golang:1.27 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests. The api and conformance modules are replaced
# with directories of this repository, so their module files must be present
# too.
COPY go.mod go.mod
COPY go.sum go.sum
COPY api/go.mod api/go.mod
COPY api/go.sum api/go.sum
COPY conformance/go.mod conformance/go.mod
COPY conformance/go.sum conformance/go.sum
# See Dockerfile.cli for why the download gets a few attempts.
RUN for attempt in 1 2 3; do \
        go mod download && break; \
        if [ "$attempt" = 3 ]; then echo "go mod download failed after 3 attempts" >&2; exit 1; fi; \
        echo "go mod download failed, retrying in 5s" >&2; \
        sleep 5; \
    done

# Copy the Go source (relies on .dockerignore to filter)
COPY . .

# Build. See Dockerfile for why GOARCH has no default.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/fakeplugin ./test/e2e/fakeplugin

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/fakeplugin .
USER 65532:65532

ENTRYPOINT ["/fakeplugin"]
