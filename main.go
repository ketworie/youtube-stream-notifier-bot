package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"youtube-stream-notifier-bot/bot"
)

var (
	tgBotToken = flag.String("tg-bot-token", "", "Telegram bot token")
	ytApiKey   = flag.String("yt-api-key", "", "YouTube Data API key")
	// TODO: add option to listen on host's port
	host = flag.String("host", "", "Host with port that is pointing to this server's 666 port. "+
		"Optional. "+
		"If missing, search.list method will be used (limited to 100 request per day)")
	debug = flag.Bool("debug", false, "Enable debug. Currently only turns on SQL output")
)

func main() {
	flag.Parse()
	if tgBotToken == nil {
		log.Fatal("Telegram bot token is not specified!")
	}
	if ytApiKey == nil {
		log.Fatal("YouTube api key is not specified!")
	}
	config := bot.Config{
		YoutubeAPIKey:    *ytApiKey,
		TelegramBotToken: *tgBotToken,
		Host:             host,
		Debug:            *debug,
	}
	ctx, cancel := context.WithCancel(context.Background())
	confirm := make(chan struct{})
	go func() {
		log.Fatal(bot.Start(ctx, config, confirm))
	}()
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)
	<-s
	cancel()
	<-confirm
}
