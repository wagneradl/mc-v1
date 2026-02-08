# Multi-stage build: compile Go binary, then copy to minimal alpine image
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o oauth-server .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /build/oauth-server /usr/local/bin/oauth-server
RUN chmod +x /usr/local/bin/oauth-server
USER 1000
EXPOSE 8090
ENTRYPOINT ["oauth-server"]
