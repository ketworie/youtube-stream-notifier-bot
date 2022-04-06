package bot

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"log"
	"net/http"
	"time"
	"youtube-stream-notifier-bot/db"
	"youtube-stream-notifier-bot/mutex"
	"youtube-stream-notifier-bot/templates"
	"youtube-stream-notifier-bot/youtube"
)

const (
	DB           = "bot"
	DBAddress    = "postgres:5432"
	DBUser       = "bot"
	DBPassword   = "makelovenotwar"
	RedisAddress = "redis:6379"
)

type Config struct {
	// YouTube Data API key
	YoutubeAPIKey string `json:"youtubeAPIKey,omitempty"`
	// Telegram bot token
	TelegramBotToken string `json:"telegramBotToken,omitempty"`
	// Host with port that is pointing to this server's 666 port.
	// Optional.
	// If missing, search.list method will be used (limited to 100 request per day)
	Host *string `json:"host,omitempty"`
	// Enable debug. Currently only turns on SQL output
	Debug bool `json:"debug,omitempty"`
}

func Start(ctx context.Context, config Config, confirm chan<- struct{}) error {
	ytService, err := youtube.NewService(config.YoutubeAPIKey)
	if err != nil {
		return err
	}
	dbService := db.New(DBAddress, DBUser, DBPassword, DB)
	mutexBuilder := mutex.NewBuilder(RedisAddress)
	if config.Debug {
		dbService.EnableDebug()
	}

	s := tele.Settings{
		Token: config.TelegramBotToken,
		Poller: &tele.LongPoller{
			Timeout: time.Second * 10,
		},
	}
	bot, err := tele.NewBot(s)
	if err != nil {
		return errors.Wrap(err, "error during creation of a new bot")
	}

	botService := NewService(ytService, dbService, mutexBuilder, bot, config.Host)

	bot.Handle("/start", botService.Start)
	bot.Handle("/add", botService.AddSubscription)
	bot.Handle("/list", botService.ListSubscribedChannels)
	bot.Handle("/remove", botService.ShowRemoveSubscription)
	bot.Handle("/help", func(context tele.Context) error {
		return context.Send(templates.Hello)
	})
	bot.Handle(tele.OnCallback, func(context tele.Context) error {
		defer func() {
			err := context.Respond()
			if err != nil {
				log.Print(err)
			}
		}()
		return botService.ProcessCallback(context)
	})

	bot.OnError = func(err error, context tele.Context) {
		log.Print(err.Error())
		err = context.Send(templates.UnexpectedError)
		if err != nil {
			log.Print(err)
		}
	}

	go func() {
		<-ctx.Done()
		bot.Stop()
		confirm <- struct{}{}
	}()

	if config.Host == nil {
		botService.StartPollingMode(ctx)
		log.Println("Started polling mode")
	} else {
		router := mux.NewRouter()
		err := botService.StartSubscriptionMode(ctx, router)
		if err != nil {
			return err
		}
		go func() {
			log.Fatal(http.ListenAndServe(":42069", router))
		}()
		log.Println("Started subscription mode")
	}
	// Blocks until stop
	bot.Start()
	return nil
}
