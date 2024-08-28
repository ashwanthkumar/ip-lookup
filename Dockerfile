FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest

RUN apk add --no-cache sqlite-libs ca-certificates

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8080

ENV IP_DATA_URL="http://example.com/dummy-ip-data.json.gz"

CMD ["./main"]
