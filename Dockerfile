# Use the official Go image as the base image
FROM golang:1.22.2 AS builder

# Install wget for health checks
RUN apk add --no-cache wget

# Set the working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Use a smaller base image for the final image
FROM alpine:latest  


# Set the working directory
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Expose the port the app runs on
EXPOSE ${APP_PORT}


# Command to run the executable
CMD ["./main"]
