FROM golang:1.23 as builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo main.go

FROM scratch
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/songs.json .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/assets ./assets
EXPOSE 3000

CMD ["./main"]