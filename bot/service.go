package bot

import (
	ctx "context"
	"fmt"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"log"
	"regexp"
	"strings"
	"time"
	"youtube-stream-notifier-bot/db"
	"youtube-stream-notifier-bot/mutex"
	"youtube-stream-notifier-bot/templates"
	"youtube-stream-notifier-bot/youtube"
)

type Service struct {
	youtube *youtube.Service
	db      *db.DB
	mb      *mutex.Builder
	bot     *tele.Bot
}

var (
	removeCallbackPattern = regexp.MustCompile("\f/remove (.+)")
	removePatternIdIndex  = 1
)

func NewService(youtube *youtube.Service, db *db.DB, mb *mutex.Builder, bot *tele.Bot) *Service {
	return &Service{
		youtube: youtube,
		db:      db,
		mb:      mb,
		bot:     bot,
	}
}

func (s *Service) Start(context tele.Context) error {
	id := context.Chat().ID
	_, err := s.db.GetChat(id)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return err
	}
	if err != nil {
		err := s.addChat(context, id)
		if err != nil {
			return err
		}
	}
	err = context.Send(templates.Hello)
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) addChat(context tele.Context, id int64) error {
	err := s.db.AddChat(db.Chat{
		Id:      id,
		Enabled: true,
	})
	if err != nil {
		sendErr := context.Send(templates.InitializationError)
		if sendErr != nil {
			log.Printf("During sending initialization error message occured error: %v", sendErr.Error())
		}
		return err
	}
	return nil
}

func (s *Service) AddSubscription(context tele.Context) error {
	id := context.Chat().ID
	_, err := s.db.GetChat(id)
	if err != nil && errors.Is(err, db.ErrNotFound) {
		err := context.Send(templates.UserNotStarted)
		if err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	data := context.Data()
	if len(data) == 0 {
		return context.Send(templates.EmptyAdd)
	}
	channel, err := s.youtube.FindChannel(ctx.Background(), data)
	if err != nil && errors.Is(err, youtube.ErrWrongUrl) {
		err := context.Send(err.Error())
		if err != nil {
			return err
		}
		return nil
	}
	if err != nil && errors.Is(err, youtube.ErrCustomUrl) {
		err := context.Send(templates.CustomUrlUnsupported)
		if err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	exists, err := s.db.ChannelExists(channel.Id)
	if err != nil {
		return errors.Wrapf(err, "cannot check if channel %v exists", channel.Id)
	}
	if !exists {
		err := s.db.AddChannel(db.Channel{
			Id:                     channel.Id,
			Title:                  channel.Title,
			RecentUploadsSectionId: channel.RecentUploadsSectionId,
			LastUpdate:             time.Now(),
		})
		if err != nil {
			return errors.Wrap(err, "cannot add channel to db")
		}
	}
	err = s.db.AddSubscription(id, channel.Id)
	if err != nil {
		return errors.Wrap(err, "cannot add user-channel link")
	}
	return context.Send(templates.AddSuccess)
}

func (s *Service) ListSubscribedChannels(context tele.Context) error {
	id := context.Chat().ID
	subscriptions, err := s.db.GetSubscribedChannels(id)
	if err != nil {
		return errors.Wrap(err, "cannot get added channels")
	}
	if len(subscriptions) == 0 {
		return context.Send(templates.NoChannels)
	}
	var subInfos []string
	for _, subscription := range subscriptions {
		subInfos = append(subInfos, fmt.Sprintf(templates.ChannelList, subscription.Title, subscription.Id))
	}
	subText := strings.Join(subInfos, "\r\n")
	return context.Send(subText)
}

func (s *Service) ShowRemoveSubscription(context tele.Context) error {
	id := context.Chat().ID
	subscriptions, err := s.db.GetSubscribedChannels(id)
	if err != nil {
		return errors.Wrap(err, "cannot get added channels")
	}
	if len(subscriptions) == 0 {
		return context.Send(templates.NoChannels)
	}
	selector := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, subscription := range subscriptions {
		dataId := fmt.Sprintf("/remove %v", subscription.Id)
		data := selector.Data(subscription.Title, dataId)
		row := selector.Row(data)
		rows = append(rows, row)
	}
	selector.Inline(rows...)
	//TODO: limit rows to 100 and add back/next buttons
	return context.Send("Select channel to remove:", selector)
}

func (s *Service) ProcessCallback(context tele.Context) error {
	chatId := context.Chat().ID
	data := context.Callback().Data
	submatch := removeCallbackPattern.FindStringSubmatch(data)
	if submatch != nil {
		err := s.RemoveSubscription(chatId, submatch[removePatternIdIndex])
		if err != nil {
			return err
		}
		return context.Send(templates.RemoveSuccess)
	}
	return nil
}

func (s *Service) RemoveSubscription(chatId int64, channelId string) error {
	return s.db.RemoveSubscription(chatId, channelId)
}

func (s *Service) StartPolling(ctx ctx.Context) {
	dbChannels := s.db.PollChannels(ctx)
	channels := transformChannels(dbChannels)
	streams := s.youtube.PollStreams(channels)
	go func() {
		for stream := range streams {
			s.notifyStream(stream)
		}
	}()
}

func transformChannels(dbChannels <-chan db.Channel) <-chan youtube.ChannelHeader {
	channels := make(chan youtube.ChannelHeader)
	go func() {
		defer close(channels)
		for channel := range dbChannels {
			channels <- youtube.ChannelHeader{
				Id:    channel.Id,
				Title: channel.Title,
			}
		}
	}()
	return channels
}

func (s *Service) notifyStream(stream youtube.StreamInfo) {
	lock := s.mb.Stream(stream.Id)
	err := lock.Lock()
	if err != nil {
		// TODO: debug log
		fmt.Println(err.Error())
		return
	}
	defer func() {
		_, err := lock.Unlock()
		if err != nil {
			log.Println(err.Error())
		}
	}()
	if s.isDone(stream) {
		return
	}
	chats, err := s.db.GetSubscribedChats(stream.Channel.Id)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	for _, chat := range chats {
		s.notifyChat(chat, stream)
	}
	err = s.db.MarkDone(stream.Id, stream.IsUpcoming)
	if err != nil {
		log.Println(err.Error())
	}
}

func (s *Service) notifyChat(chat db.Chat, stream youtube.StreamInfo) {
	lock := s.mb.LockStreamChat(stream.Id, chat.Id)
	// Stream is marked as done only when all chats are notified.
	// So in case of a sudden shutdown we need to mark chats as notified.
	// We do not need to unlock this lock.
	err := lock.Lock()
	if err != nil {
		// TODO: debug log
		fmt.Println(err.Error())
		return
	}
	var message string
	url := fmt.Sprintf("https://youtube.com/watch?v=%v", stream.Id)
	if stream.IsUpcoming {
		message = fmt.Sprintf(templates.Upcoming, stream.Channel.Title, stream.ScheduledStart.String(), url)
	} else {
		message = fmt.Sprintf(templates.Live, stream.Channel.Title, url)
	}
	_, err = s.bot.Send(tele.ChatID(chat.Id), message)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (s *Service) isDone(stream youtube.StreamInfo) bool {
	if stream.IsUpcoming {
		isUpcomingDone, err := s.db.IsUpcomingDone(stream.Id)
		if err != nil {
			fmt.Println(err.Error())
			return true
		}
		if isUpcomingDone {
			return true
		}
	} else {
		isLiveDone, err := s.db.IsLiveDone(stream.Id)
		if err != nil {
			fmt.Println(err.Error())
			return true
		}
		if isLiveDone {
			return true
		}
	}
	return false
}