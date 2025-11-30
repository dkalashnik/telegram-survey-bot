# Telegram Survey Bot Helm Repository

Helm chart repo for deploying [`dkalashnik/telegram-survey-bot`](https://github.com/dkalashnik/telegram-survey-bot).

## Add the repo

```bash
helm repo add telegram-survey-bot https://dkalashnik.github.io/telegram-survey-bot/charts
helm repo update
```

## Install/upgrade

The chart requires:
- `env.telegramBotToken` (Telegram bot token)
- `env.targetUserId` (numeric Telegram user ID to receive forwarded answers)
- `env.recordConfig` (YAML for `record_config.yaml`)

Example (inline config):

```bash
helm upgrade --install telegram-survey-bot telegram-survey-bot/telegram-survey-bot \
  --version 0.0.1-rc.5 \
  --set env.telegramBotToken=YOUR_TOKEN \
  --set env.targetUserId=123456789 \
  --set-file env.recordConfig=record_config.yaml
```

If you already have a secret with `TELEGRAM_BOT_TOKEN` and `TARGET_USER_ID`, set `env.secretRef` to its name and omit the token/user values.

## Chart source

Charts are published from the `main` branch of the GitHub repo; this `gh-pages` branch hosts the packaged charts and `index.yaml`.
