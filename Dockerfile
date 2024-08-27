FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN GOOS=linux go build -a -o main .

FROM alpine:latest

RUN apk add --no-cache sqlite-libs ca-certificates

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8080

ENV IP_DATA_URL="http://example.com/dummy-ip-data.json.gz"

CMD ["./main"]
