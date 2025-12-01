package main

import (
	"context"
	"github.com/dkalashnik/telegram-survey-bot/pkg/bot"
	"github.com/dkalashnik/telegram-survey-bot/pkg/bot/telegramadapter"
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/fsm"
	"github.com/dkalashnik/telegram-survey-bot/pkg/fsm/questions"
	"github.com/dkalashnik/telegram-survey-bot/pkg/ports/botport"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	questions.RegisterBuiltins()

	cfgPath := "record_config.yaml"
	if err := config.LoadConfig(cfgPath); err != nil {
		log.Panicf("Failed to load configuration: %v", err)
	}
	log.Println("Configuration loaded successfully.")

	loadedConfig := config.GetConfig()

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Panic("TELEGRAM_BOT_TOKEN environment variable not set")
	}
	if err := config.LoadTargetUserIDFromEnv(); err != nil {
		log.Panicf("Failed to read TARGET_USER_ID: %v", err)
	}

	botClient, err := bot.NewClient(botToken)
	if err != nil {
		log.Panicf("Failed to initialize bot client: %v", err)
	}
	log.Printf("Authorized on account %s", botClient.Self.UserName)

	botPort, err := telegramadapter.New(botClient, log.Default())
	if err != nil {
		log.Panicf("Failed to create telegram adapter: %v", err)
	}

	notifyTargetOnStartup(botPort)

	fsmCreator := fsm.NewFSMCreator()
	stateStore := state.NewStore(fsmCreator)
	updates := botClient.GetUpdatesChan(60)
	log.Println("Starting update processing...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("Shutdown signal received...")
		cancel()
	}()

	for {
		select {
		case update := <-updates:
			if update.UpdateID == 0 {
				continue
			}
			go fsm.HandleUpdate(ctx, update, botPort, loadedConfig, stateStore)
		case <-ctx.Done():
			log.Println("Stopping update processing loop...")
			return
		}
	}
}

func notifyTargetOnStartup(botPort botport.BotPort) {
	targetUserID := config.GetTargetUserID()
	if targetUserID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := botPort.SendMessage(ctx, targetUserID, "Бот запущен и готов принимать ответы.", nil)
	if err != nil {
		log.Printf("[main] Failed to send startup notification to %d: %v", targetUserID, err)
		return
	}
	log.Printf("[main] Startup notification sent to %d", targetUserID)
}
