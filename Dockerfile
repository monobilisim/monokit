FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

# Install make and other build dependencies
RUN apk add --no-cache make git bash binutils

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application using make and target architecture
ARG TARGETARCH
RUN GOARCH=$TARGETARCH make

# Create the final image
FROM alpine:3.21

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/bin/monokit /usr/local/bin/monokit

# Copy the configuration files
COPY --from=builder /app/config /etc/mono/

# Set the command to just be monokit
CMD ["monokit"]
