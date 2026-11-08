# Build the API and a separate migration runner. Build context: ./backend
FROM golang:1.27.3-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM golang:1.27.3-bookworm AS goose-builder
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.26.0

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
