FROM golang:1.22.2-alpine AS builder

# Install git and development dependencies
RUN apk add --no-cache git

# Install Air for hot-reloading
RUN go install github.com/cosmtrek/air@latest

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code and .air.toml
COPY . .

# Verify the presence of .air.toml
RUN ls -la && cat .air.toml

# Build the application
RUN go build -o main .

# Expose port 8080 to the outside world
EXPOSE 8080

# Copy start script
COPY start.sh .

# Make start script executable
RUN chmod +x start.sh

# Use start script as entrypoint
CMD ["./start.sh"]

# Command to run the application with Air
CMD ["air", "-c", ".air.toml"]
