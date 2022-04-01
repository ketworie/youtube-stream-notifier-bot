package bot

import (
	"context"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"log"
	"os"
	"time"
	"youtube-stream-notifier-bot/db"
	"youtube-stream-notifier-bot/mutex"
	"youtube-stream-notifier-bot/templates"
	"youtube-stream-notifier-bot/youtube"
)

const (
	DB           = "bot"
	DBAddress    = ":5432"
	DBUser       = "bot"
	DBPassword   = "makelovenotwar"
	RedisAddress = ":6379"
)

func Start(ctx context.Context, confirm chan<- struct{}) error {
	ytService, err := youtube.NewService()
	if err != nil {
		return err
	}
	dbService, err := db.New(DBAddress, DBUser, DBPassword, DB)
	if err != nil {
		return err
	}
	mutexBuilder := mutex.NewBuilder(RedisAddress)

	s := tele.Settings{
		Token: os.Getenv("TG_YT_BOT_TOKEN"),
		Poller: &tele.LongPoller{
			Timeout: time.Second * 10,
		},
	}
	bot, err := tele.NewBot(s)
	if err != nil {
		return errors.Wrap(err, "error during creation of a new bot")
	}

	botService := NewService(ytService, dbService, mutexBuilder, bot)

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
	}()
	botService.StartPolling(ctx)
	bot.Start()
	confirm <- struct{}{}
	return nil
}
