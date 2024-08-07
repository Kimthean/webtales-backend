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

# Copy the source code
COPY . .

# Print directory contents and file info
RUN echo "Directory contents:" && \
    ls -la && \
    echo ".air.toml contents:" && \
    cat air.toml || echo ".air.toml not found"


# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the application with Air
RUN go clean -modcache
CMD ["air", "-c", ".air.toml"]