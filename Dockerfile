# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/server ./cmd/server

# Run stage
FROM alpine:3.20
RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser
WORKDIR /app
COPY --from=builder /bin/server /bin/server
USER appuser
EXPOSE 8080
ENTRYPOINT ["/bin/server"]
