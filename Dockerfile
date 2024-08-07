# Use an official Go runtime as a parent image
FROM golang:1.18-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the local package files to the container's workspace.
ADD . /app

# Build the Go app
RUN go build -o main .

# Run the command by default when the container starts.
CMD ["/app/main"]

# Document that the service listens on port 8080.
EXPOSE 8080
