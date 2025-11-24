# Multi-stage build for fs-vectorize with custom ChromaDB client
FROM golang:1.24-alpine AS builder

# Install basic build dependencies
RUN apk --no-cache add \
  git \
  ca-certificates \
  tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application (CGO disabled since we use custom HTTP-based ChromaDB client)
RUN CGO_ENABLED=0 GOOS=linux go build -a \
  -ldflags '-w -s' \
  -o idaes ./cmd/main.go

# Final stage - Use Alpine for smaller container size
FROM alpine:3.18

# Install dependencies
RUN apk --no-cache add \
  ca-certificates \
  tzdata \
  libstdc++ \
  curl

# Create non-root user
RUN addgroup -g 1001 appgroup && \
  adduser -u 1001 -G appgroup -s /bin/sh -D appuser

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/idaes .

# Copy web static files
COPY --from=builder /app/web ./web

# Set ownership of the app directory
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port (WebUI runs on 8081)
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8081/api/v1/health || exit 1

# Run the application
CMD ["/app/idaes", \
  "-chroma-url", "http://chromadb:8000", \
  "-ollama-url", "http://ollama:11434", \
  "-host", "0.0.0.0", \
  "-port", "8081", \
  "-enable_web", "true", \
  "-enable-display", "false", \
  "-enable-gpio", "false", \
  "-enable_idaes", "true", \
  "-worker_count", "4", \
  "-max-concurrent-docs", "4", \
  "-parallel-processing", "true", \
  "-llm-timeout", "120s", \
  "-llm-retry-attempts", "3", \
  "-debug"]