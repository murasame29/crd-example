FROM golang:1.22.0 AS builder
WORKDIR /build

COPY main.go main.go
COPY go.mod ./
COPY go.sum ./

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o server

FROM alpine AS app
WORKDIR /app

COPY --from=builder /build/server /app/server

CMD ["/app/server"]