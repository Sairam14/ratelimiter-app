FROM golang:1.21.7-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o ratelimiter-app ./cmd/ratelimiter-app

FROM gcr.io/distroless/base-debian12:latest

WORKDIR /root/

COPY --from=builder /app/ratelimiter-app .

CMD ["./ratelimiter-app"]