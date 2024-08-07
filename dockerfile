FROM golang:1.22.2-alpine

RUN apk add --no-cache iputils curl bind-tools

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

EXPOSE 8080
# Command to run the executable
CMD ["./main"]
