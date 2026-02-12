FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o relay .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /build/relay /usr/local/bin/relay
EXPOSE 8443
ENTRYPOINT ["relay"]
