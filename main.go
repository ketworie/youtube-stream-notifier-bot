package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"youtube-stream-notifier-bot/bot"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	confirm := make(chan struct{})
	go func() {
		log.Fatal(bot.Start(ctx, confirm))
	}()

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)
	<-s
	cancel()
	<-confirm
}
