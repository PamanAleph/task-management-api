FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/bin/api ./api
COPY --from=builder /app/migrations ./migrations
EXPOSE 8080
ENTRYPOINT ["./api"]
