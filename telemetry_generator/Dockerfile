FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY main.go .
RUN go mod init telemetry
RUN go get github.com/twmb/franz-go/pkg/kgo
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o telemetry_gen .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/telemetry_gen .
CMD ["./telemetry_gen"]
