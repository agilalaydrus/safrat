FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server
COPY --from=build /worker /worker
EXPOSE 8080
USER nonroot:nonroot
# CMD, not ENTRYPOINT — docker-compose.prod.yml's worker service overrides
# this with `command: ["/worker"]` to reuse the same image; ENTRYPOINT
# would only ever prepend arguments to /server instead of replacing it.
CMD ["/server"]
