# Build the API and a separate migration runner. Build context: ./backend
FROM golang:1.27.1-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM golang:1.27.1-bookworm AS goose-builder
# Exclude every driver except PostgreSQL to avoid compiling large unused database clients.
# Keep this migration binary lean on smaller ARM64 VPS machines.
RUN go install -tags='no_azuresql no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' github.com/pressly/goose/v3/cmd/goose@v3.26.0

FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=builder /out/api /api
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/api"]

FROM debian:bookworm-slim AS migrate
COPY --from=goose-builder /go/bin/goose /usr/local/bin/goose
COPY migrations /migrations
USER 65532:65532
ENTRYPOINT ["goose"]
