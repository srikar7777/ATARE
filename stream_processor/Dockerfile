FROM golang:1.22-alpine AS builder

WORKDIR /app
RUN apk add --no-cache gcc musl-dev

COPY main.go .
RUN go mod init processor
RUN go get github.com/twmb/franz-go/pkg/kgo
RUN go get github.com/redis/go-redis/v9
RUN go get github.com/mattn/go-sqlite3
RUN go mod tidy
RUN CGO_ENABLED=1 GOOS=linux go build -o processor .

FROM alpine:latest
RUN apk add --no-cache sqlite-libs
WORKDIR /app
COPY --from=builder /app/processor .
CMD ["./processor"]
