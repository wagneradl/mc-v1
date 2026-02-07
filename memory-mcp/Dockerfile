FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o memory-mcp .

FROM alpine:3.20
RUN adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /build/memory-mcp .
RUN mkdir -p data/projects data/archive && chown -R appuser:appuser data
USER appuser
ENTRYPOINT ["./memory-mcp"]
