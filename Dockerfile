# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/telegram-survey-bot ./main.go

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /app/telegram-survey-bot /app/telegram-survey-bot
COPY record_config.yaml /app/record_config.yaml

ENV TELEGRAM_BOT_TOKEN="" \
    TARGET_USER_ID=""

ENTRYPOINT ["/app/telegram-survey-bot"]
