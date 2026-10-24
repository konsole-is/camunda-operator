# Build the Git server of the kind harness of the fleet manager. The CI job
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
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/gitserver ./test/e2e/gitserver

# The server runs git http-backend, and the apply loop of the harness runs
# git in this container, so the image needs git and not only the binary.
FROM debian:trixie-slim
RUN apt-get update \
    && apt-get install --no-install-recommends -y git ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && test -x "$(git --exec-path)/git-http-backend"
COPY --from=builder /workspace/gitserver /gitserver
USER 65532:65532
ENV HOME=/tmp

ENTRYPOINT ["/gitserver"]
