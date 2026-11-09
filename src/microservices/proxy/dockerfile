FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY main.go .

RUN go mod init proxy && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o proxy .

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/proxy .

EXPOSE 8000
CMD ["./proxy"]