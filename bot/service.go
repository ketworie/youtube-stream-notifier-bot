package bot

import (
	ctx "context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"youtube-stream-notifier-bot/db"
	"youtube-stream-notifier-bot/mutex"
	"youtube-stream-notifier-bot/templates"
	"youtube-stream-notifier-bot/timezone"
	"youtube-stream-notifier-bot/youtube"
)

type Service struct {
	youtube       *youtube.Service
	db            *db.DB
	mb            *mutex.Builder
	tz            *timezone.Service
	bot           *tele.Bot
	subscribeHost *string
	lc            locationCache
}

type locationCache map[string]*time.Location

func (lc locationCache) get(timeZone string) (*time.Location, error) {
	if l, ok := lc[timeZone]; ok {
		return l, nil
	}
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return nil, err
	}
	lc[timeZone] = location
	return location, nil
}

var (
	removeCallbackPattern   = regexp.MustCompile("\f/remove id:(.+);t:(.+)")
	removePatternIdIndex    = 1
	removePatternTitleIndex = 2
	subscribeClient         = http.Client{Timeout: subscribeTimeout}
)

const (
	subscribeTimeout = time.Second * 10
)

func NewService(
	youtube *youtube.Service,
	db *db.DB,
	mb *mutex.Builder,
	tz *timezone.Service,
	bot *tele.Bot,
	subscribeHost *string,
) *Service {
	return &Service{
		youtube:       youtube,
		db:            db,
		mb:            mb,
		tz:            tz,
		bot:           bot,
		subscribeHost: subscribeHost,
		lc:            make(map[string]*time.Location),
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
	err := s.db.AddChat(
		db.Chat{
			Id:      id,
			Enabled: true,
		},
	)
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
	if err != nil && errors.Is(err, youtube.ErrBadUrl) {
		err := context.Send(err.Error())
		if err != nil {
			return err
		}
		return nil
	}
	if err != nil && errors.Is(err, youtube.ErrUnsupportedUrl) {
		err := context.Send(templates.UrlUnsupported)
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
		err := s.db.AddChannel(
			db.Channel{
				Id:         channel.Id,
				Title:      channel.Title,
				LastUpdate: time.Now(),
			},
		)
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
	channels, err := s.db.GetSubscribedChannels(id)
	if err != nil {
		return errors.Wrap(err, "cannot get added channels")
	}
	if len(channels) == 0 {
		return context.Send(templates.NoChannels)
	}
	selector := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, channel := range channels {
		dataId := fmt.Sprintf("/remove id:%v;t:%v", channel.Id, channel.Title)
		data := selector.Data(channel.Title, dataId)
		row := selector.Row(data)
		rows = append(rows, row)
	}
	selector.Inline(rows...)
	// TODO: limit rows to 100 and add back/next buttons
	return context.Send("Select channel to remove:", selector)
}

func (s *Service) OnLocation(context tele.Context) error {
	location := context.Message().Location
	if location == nil {
		return errors.New("location is empty")
	}

	zone, err := s.tz.GetTimeZone(fmt.Sprintf("%f", location.Lat), fmt.Sprintf("%f", location.Lng))
	if err != nil {
		return errors.Wrapf(err, "error on getting timezone by location lat: %v, lng: %v", location.Lat, location.Lng)
	}
	err = s.db.SetChatTimeZone(context.Chat().ID, zone)
	if err != nil {
		return err
	}
	return context.Send(fmt.Sprintf(templates.TimeZoneSuccess, zone))
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
		return context.Send(fmt.Sprintf(templates.RemoveSuccess, submatch[removePatternTitleIndex]))
	}
	return errors.New("couldn't get channel data from remove callback")
}

func (s *Service) RemoveSubscription(chatId int64, channelId string) error {
	return s.db.RemoveSubscription(chatId, channelId)
}

func (s *Service) StartPollingMode(ctx ctx.Context) {
	dbChannels := s.db.PollChannels(ctx, false)
	channels := transformChannels(dbChannels)
	streams := s.youtube.PollStreams(channels)
	go func() {
		for stream := range streams {
			s.notifyAboutStream(stream)
		}
	}()
}

func (s *Service) StartSubscriptionMode(ctx ctx.Context, router *mux.Router) error {
	if s.subscribeHost == nil {
		return errors.New("Subscribe host is not specified")
	}
	channels := s.db.PollChannels(ctx, true)
	go startSubscriptionRenewal(*s.subscribeHost, channels)
	router.Methods(http.MethodGet).Path("/video").HandlerFunc(s.db.HandleConfirmSubscription)
	streams := make(chan youtube.StreamInfo)
	router.Methods(http.MethodPost).Path("/video").HandlerFunc(s.getFeedHandler(streams))
	go func() {
		for stream := range streams {
			s.notifyAboutStream(stream)
		}
	}()
	return nil
}

func startSubscriptionRenewal(subscriptionHost string, channels <-chan db.Channel) {
	for channel := range channels {
		err := subscribe(subscriptionHost, channel.Id)
		if err != nil {
			log.Printf("error when trying to subscribe to channel: %v", err.Error())
		}
	}
}

func subscribe(subscriptionHost string, channelId string) error {
	topic := fmt.Sprintf(youtube.HubTopicFormat, channelId)
	callback := fmt.Sprintf(youtube.HubSubscribePathURLFormat, subscriptionHost)
	values := url.Values{}
	values.Set(youtube.HubTopic, topic)
	values.Set(youtube.HubCallback, callback)
	values.Set(youtube.HubVerify, youtube.HubVerifyAsync)
	values.Set(youtube.HubMode, youtube.HubModeSubscribe)
	response, err := subscribeClient.PostForm(youtube.HubYouTubeURL, values)
	if err != nil {
		return errors.Wrapf(err, "unable to make sunscribe request")
	}
	body := response.Body
	defer func() {
		err := body.Close()
		if err != nil {
			log.Printf("error when closing the body: %v", err.Error())
		}
	}()
	code := response.StatusCode
	if code < 200 || code > 299 {
		body, err := ioutil.ReadAll(body)
		if err != nil {
			return errors.Errorf("unexpected status during subscription %v; can't read body: %v", code, err.Error())
		}
		return errors.Errorf("unexpected status during subscription %v; body: %v", code, string(body))
	}
	return nil
}

func transformChannels(dbChannels <-chan db.Channel) <-chan youtube.ChannelInfo {
	channels := make(chan youtube.ChannelInfo)
	go func() {
		defer close(channels)
		for channel := range dbChannels {
			channels <- youtube.ChannelInfo{
				Id:    channel.Id,
				Title: channel.Title,
			}
		}
	}()
	return channels
}

func (s *Service) notifyAboutStream(stream youtube.StreamInfo) {
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
		s.notifyChatAboutStream(chat, stream)
	}
	err = s.db.MarkDone(stream.Id, stream.IsUpcoming)
	if err != nil {
		log.Println(err.Error())
	}
}

func (s *Service) notifyChatAboutStream(chat db.Chat, stream youtube.StreamInfo) {
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
	videoURL := fmt.Sprintf("https://youtube.com/watch?v=%v", stream.Id)
	if stream.IsUpcoming {
		var scheduledStartTime string
		if chat.TimeZone != nil {
			location, err := s.lc.get(*chat.TimeZone)
			if err != nil {
				log.Printf("Unable to get location for time zone: %v", *chat.TimeZone)
				location = time.UTC
			}
			scheduledStartTime = stream.ScheduledStart.In(location).Format(time.RFC850)
		} else {
			scheduledStartTime = stream.ScheduledStart.String()
		}
		message = fmt.Sprintf(templates.Upcoming, stream.Channel.Title, scheduledStartTime, videoURL)
		// 10% chance to display time zone help
		if chat.TimeZone == nil && rand.Intn(10) == 0 {
			message = fmt.Sprintf("%v\r\n%v", message, templates.SetTimeZoneHelp)
		}
	} else {
		message = fmt.Sprintf(templates.Live, stream.Channel.Title, videoURL)
	}
	// TODO: Disable chat if bot blocked
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
