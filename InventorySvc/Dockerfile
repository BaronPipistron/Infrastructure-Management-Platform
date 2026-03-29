FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/inventory-svc ./cmd/inventory-svc

FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser

COPY --from=builder /out/inventory-svc /usr/local/bin/inventory-svc

ENV APP_CONFIG_PATH=/etc/inventory-svc/configs/appsettings.Prod.yml
VOLUME ["/etc/inventory-svc"]

EXPOSE 8080

USER appuser

ENTRYPOINT ["/usr/local/bin/inventory-svc"]
