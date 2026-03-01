# Stage 1 — Build
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -o repbot-go .


# Stage 2 — Runtime
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/repbot-go /app/repbot-go

ENV REPBOT_DB=/data/rep.db

VOLUME ["/data", "/etc/repbot"]

ENTRYPOINT ["/app/repbot-go"]
