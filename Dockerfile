FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -trimpath -ldflags="-s -w" -o main ./cmd/app/main.go

FROM alpine:3.22

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app \
    && mkdir -p /app/uploads/avatars && chown -R app:app /app

WORKDIR /app

COPY --from=builder /app/main .

USER app

EXPOSE 8090

CMD ["./main"]
