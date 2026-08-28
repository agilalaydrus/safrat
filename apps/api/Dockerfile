FROM golang:1.25-alpine AS build
WORKDIR /app
COPY apps/api/go.mod apps/api/go.sum apps/api/
RUN cd apps/api && go mod download
COPY . .

# Install buf
RUN go install github.com/bufbuild/buf/cmd/buf@latest

# Install sqlc — pinned, downloaded as a prebuilt binary instead of
# `go install` so it doesn't need Go >= 1.26 (sqlc v1.31.1+'s requirement)
# on this image's golang:1.25-alpine toolchain.
RUN apk add --no-cache curl
RUN curl -fsSL https://github.com/sqlc-dev/sqlc/releases/download/v1.31.1/sqlc_1.31.1_linux_amd64.tar.gz | tar -xz -C /usr/local/bin sqlc

# Generate proto + sqlc
RUN cd /app/proto && buf generate
RUN cd /app/apps/api && sqlc generate

RUN cd apps/api && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /server ./cmd/server
RUN cd apps/api && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /worker ./cmd/worker
# One-time maintenance job, run manually against a running deployment
# (`docker compose run --rm api /storefront-backfill`). Shipped in the same
# image so it always matches the schema the deployed API expects.
RUN cd apps/api && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /storefront-backfill ./cmd/storefront-backfill
RUN mkdir -p /app/uploads/documents

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server
COPY --from=build /worker /worker
COPY --from=build /storefront-backfill /storefront-backfill
# Seed the named production volume with a directory owned by distroless'
# nonroot user. Without this, uploads target / in a read-only/non-writable
# container and disappear whenever the image is replaced.
COPY --from=build --chown=65532:65532 /app/uploads /uploads
EXPOSE 8080
USER nonroot:nonroot
# CMD, not ENTRYPOINT — docker-compose.prod.yml's worker service overrides
# this with `command: ["/worker"]` to reuse the same image; ENTRYPOINT
# would only ever prepend arguments to /server instead of replacing it.
CMD ["/server"]
