# Dockerfile
FROM golang:1.22.2-alpine AS builder

# Install git and development dependencies
RUN apk add --no-cache git

# Install Air for hot-reloading
RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Print directory contents and file info
RUN echo "Directory contents:" && \
    ls -la && \
    echo "air.toml contents:" && \
    cat air.toml || echo "air.toml not found"

# Build the application
RUN go build -o main .

# Expose port 8080 to the outside world
EXPOSE 8080

# Create a shell script to run the application
RUN echo '#!/bin/sh' > run.sh && \
    echo 'echo "Current directory: $(pwd)"' >> run.sh && \
    echo 'echo "Directory contents:"' >> run.sh && \
    echo 'ls -la' >> run.sh && \
    echo 'if [ -f "air.toml" ]; then' >> run.sh && \
    echo '    echo "Running with Air"' >> run.sh && \
    echo '    air -c air.toml' >> run.sh && \
    echo 'else' >> run.sh && \
    echo '    echo "air.toml not found, running Go application directly"' >> run.sh && \
    echo '    ./main' >> run.sh && \
    echo 'fi' >> run.sh && \
    chmod +x run.sh

# Use the shell script as the entry point
CMD ["./run.sh"]