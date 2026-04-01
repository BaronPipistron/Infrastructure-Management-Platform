FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/parser-svc ./cmd/parser-svc

FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser

COPY --from=builder /out/parser-svc /usr/local/bin/parser-svc

ENV APP_CONFIG_PATH=/etc/parser-svc/configs/appconfig.Prod.yml
VOLUME ["/etc/parser-svc"]

EXPOSE 8080

USER appuser

ENTRYPOINT ["/usr/local/bin/parser-svc"]
