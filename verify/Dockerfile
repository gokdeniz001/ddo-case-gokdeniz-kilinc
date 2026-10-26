FROM golang:1.22-alpine

WORKDIR /app

COPY verify/main.go ./main.go

RUN go build -o /app/verify ./main.go

ENTRYPOINT ["/app/verify"]
