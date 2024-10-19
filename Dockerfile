# Stage1: Build the Go binary
FROM amd64/golang:1.23 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod donwload

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o /app/bin/api ./cmd/api

# Stage2: Create a lightweight image to run the application
FROM amd64/alpine:3.14

# Install necessary dependencies
RUN apk --no-cache add ca-certificates

# Set up User
RUN adduser -D -g '' greenlight

# Set the working directory
WORKDIR /home/greenlight

# Copy the compiled Go bianry from the builder stage
COPY --from=builder /app/bin/api .

# Change ownership to non-root user
RUN chown -R greenlight:greenlight /home/greenlight

USER greenlight

#
CMD ["./api"]
