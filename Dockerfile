# Use the official Golang image as the base image
FROM golang:1.23.0

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application
RUN go build -o main .

# Expose the application on port 8080
EXPOSE 8080

# Run the Go application
CMD ["./main"]

