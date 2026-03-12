# Stage 1: Build the statically linked Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git and other build dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies.
RUN go mod download

# Copy the source code
COPY . .

# Build the application statically linked
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o orchestrator ./cmd/bot/main.go

# Stage 2: Create a minimal runner container using scratch
FROM scratch

# Import certs so HTTPS works (needed for external API calls to Gemini)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Import timezone data so time.LoadLocation works in scratch
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /zoneinfo.zip
ENV ZONEINFO=/zoneinfo.zip

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/orchestrator /app/orchestrator

# Command to run the executable
ENTRYPOINT ["/app/orchestrator"]
