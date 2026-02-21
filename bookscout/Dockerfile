FROM golang:1.25.7-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bookscout cmd/worker/main.go

FROM gcr.io/distroless/static-debian12

WORKDIR /

COPY --from=builder /app/bookscout /bookscout

ENV BS_LOG_LEVEL=info

ENTRYPOINT ["/bookscout"]
