FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod init events-service || true
RUN go get github.com/segmentio/kafka-go

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o events .

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/events .

EXPOSE 8082
CMD ["./events"]