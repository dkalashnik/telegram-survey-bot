package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"telegramsurveylog/pkg/bot"
	"telegramsurveylog/pkg/bot/telegramadapter"
	"telegramsurveylog/pkg/config"
	"telegramsurveylog/pkg/fsm"
	"telegramsurveylog/pkg/fsm/questions"
	"telegramsurveylog/pkg/state"
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
