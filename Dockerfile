FROM golang:1.23 AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .
RUN touch /app/.env

FROM --platform=linux/amd64 scratch
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/assets ./assets
COPY --from=builder /app/.env .
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

EXPOSE 3000

CMD ["./main"]