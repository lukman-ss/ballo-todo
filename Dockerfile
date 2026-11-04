# Build stage — CGO_ENABLED=0: modernc.org/sqlite is pure Go, no CGO needed
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/server ./cmd/server

# Run stage
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

WORKDIR /app
# Copy binary
COPY --from=builder /bin/server /bin/server
# Copy templates and static assets (served from working directory at runtime)
COPY --from=builder /app/internal/web/templates ./internal/web/templates
COPY --from=builder /app/internal/web/static ./internal/web/static

# Data directory for SQLite — mount a volume here for persistence
RUN mkdir -p /app/data && chown -R appuser:appgroup /app
USER appuser

EXPOSE 8080

# DB_PATH and TZ can be overridden via env vars
ENV DB_PATH=/app/data/ballo-todo.db
ENV TZ=Asia/Jakarta
ENV TEMPLATE_DIR=/app/internal/web/templates

ENTRYPOINT ["/bin/server"]
