# Stage 1: Build the application
FROM golang:1.26-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# -ldflags "-w -s" strips debugging information, making the binary smaller
# CGO_ENABLED=0 creates a statically-linked binary
RUN CGO_ENABLED=0 go build -ldflags "-w -s" -o /gocrawl ./cmd/main.go

# Stage 2: Create the final, minimal image
FROM alpine:3.21

RUN apk upgrade --no-cache

# Copy the built binary from the builder stage
COPY --from=builder /gocrawl /gocrawl

# Expose the port the application runs on
EXPOSE 8151

# Set the entrypoint for the container
ENTRYPOINT ["/gocrawl"]

