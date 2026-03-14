FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev vips-dev ffmpeg

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./cmd/api

# Runtime
FROM alpine:3.20

RUN apk add --no-cache vips ffmpeg ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/server .
# Migrations are embedded in the binary — no separate copy needed

EXPOSE 8080
CMD ["./server"]
