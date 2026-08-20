FROM golang:1.25-alpine AS build
WORKDIR /app
COPY apps/api/go.mod apps/api/go.sum apps/api/
RUN cd apps/api && go mod download
COPY . .

# Install buf
RUN go install github.com/bufbuild/buf/cmd/buf@latest

# Install sqlc — pinned (not @latest): sqlc v1.31.1+ requires Go >= 1.26,
# newer than this image's golang:1.25-alpine. v1.27.0 matches the version
# .github/workflows/deploy.yml's CI job installs.
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0

# Generate proto + sqlc
RUN cd /app/proto && buf generate
RUN cd /app/apps/api && sqlc generate

RUN cd apps/api && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /server ./cmd/server
RUN cd apps/api && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server
COPY --from=build /worker /worker
EXPOSE 8080
USER nonroot:nonroot
# CMD, not ENTRYPOINT — docker-compose.prod.yml's worker service overrides
# this with `command: ["/worker"]` to reuse the same image; ENTRYPOINT
# would only ever prepend arguments to /server instead of replacing it.
CMD ["/server"]
