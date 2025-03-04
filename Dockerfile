FROM golang:1.23.4-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o k8sgptclient main.go

RUN go install github.com/gptscript-ai/gptscript@latest

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/k8sgptclient .
COPY --from=builder /go/bin/gptscript /usr/local/bin/gptscript
RUN chmod +x /usr/local/bin/gptscript

ENTRYPOINT ["./k8sgptclient"]