# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download && go mod verify
RUN go build -o go-jobdispatch ./cmd/jobdispatch

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/go-jobdispatch .
EXPOSE 8080
CMD ["./go-jobdispatch"]
