package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"youtube-stream-notifier-bot/bot"
)

func main() {
	file, err := os.ReadFile("./config.json")
	if err != nil {
		log.Fatalf("unable to read config file: %v", err.Error())
		return
	}

	var c bot.Config
	err = json.Unmarshal(file, &c)
	if err != nil {
		log.Fatalf("unable to unmarshall config file: %v", err.Error())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	confirm := make(chan struct{})
	go func() {
		log.Fatal(bot.Start(ctx, c, confirm))
	}()
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)
	<-s
	cancel()
	<-confirm
}
