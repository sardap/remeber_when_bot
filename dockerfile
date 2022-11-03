FROM golang:1.18.8-alpine3.16 as builder

WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY main.go .
RUN go build -o main .

FROM alpine:3.16.2

RUN apk add --no-cache tzdata

COPY --from=builder /app/main /app/main

WORKDIR /app
ENTRYPOINT [ "./main" ]
