# Use an official Golang runtime as a parent image
FROM golang:latest

# Set the working directory inside the container
WORKDIR "/usr/local/go/src/couchbase-emx"

# Copy the local package files to the container's workspace
COPY . .
COPY exporter ./exporter

# Set the PORT environment variable with a default value
ENV PORT=9876

# Expose the port the app runs on
EXPOSE $PORT

# Copy the Go module files
COPY go.mod .
COPY go.sum .
# RUN go mod download

# ARGs for build configuration
ARG GOOS
ARG GOARCH

# Build a statically linked binary
RUN CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -a -ldflags '-extldflags "-static"' -o couchbase_emx ./exporter

# Command to run the executable with dynamic environment variables
# CMD ["./couchbase_emx"]
ENTRYPOINT ["./couchbase_emx"]

