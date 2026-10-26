FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download && go mod verify
COPY backend/ ./
RUN CGO_ENABLED=0 go test -c -o /integration.test ./tests
COPY verify/main.go /verify.go
RUN CGO_ENABLED=0 go build -o /verify /verify.go
FROM alpine:3.19
RUN adduser -D verifier
WORKDIR /app/tests
COPY --from=builder /integration.test /app/integration.test
COPY --from=builder /verify /app/verify
COPY backend/migrations/ /app/migrations/
USER verifier
ENTRYPOINT ["sh", "-c", "/app/integration.test -test.v && /app/verify"]
