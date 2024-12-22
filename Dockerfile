# Start from the official Go image
FROM golang:1.22.3

# Set environment variables
ENV GO111MODULE=on
ENV GIN_MODE=release

# Create and set the working directory
WORKDIR /app

# Copy the Go modules and install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application
RUN go build -o main .

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["./main"]
