FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/api /api
COPY --from=builder /app/migrations /migrations

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/api"]
