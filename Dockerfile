FROM golang:1.25-alpine

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the application
# Pastikan path ini sesuai dengan struktur project Anda
RUN go build -o /app/main ./cmd/app/main.go

# Expose port
EXPOSE 8090

# Run the application
CMD ["/app/main"]