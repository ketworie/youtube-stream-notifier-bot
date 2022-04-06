package youtube

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	ytApi "google.golang.org/api/youtube/v3"
	"regexp"
	"time"
)

// TODO: separate /c and /user to get customUrl or id
var urlPattern = regexp.MustCompile("(https?://)?(www\\.)?youtu((\\.be)|(be\\..{2,5}?))/(channel/(UC[\\w-]{21}[AQgw])|(c/|user/)?([\\w-]+))")

const (
	channelIdIndex  = 7
	customNameIndex = 9
)

var (
	snippetPart              = []string{"snippet"}
	livestreamingDetailsPart = []string{"liveStreamingDetails"}
	ErrWrongUrl              = errors.New("unable to parse url")
	ErrCustomUrl             = errors.New("custom url is not supported")
	ErrNotStream             = errors.New("video is not a live or upcoming stream")
)

type Service struct {
	yt *ytApi.Service
}

func NewService(apiKey string) (*Service, error) {
	service, err := ytApi.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &Service{service}, nil
}

func (s *Service) FindChannel(ctx context.Context, url string) (ChannelInfo, error) {
	submatch := urlPattern.FindStringSubmatch(url)
	if submatch == nil {
		return ChannelInfo{}, ErrWrongUrl
	}
	call := s.yt.Channels.List(snippetPart).Context(ctx).MaxResults(1)
	channelId := submatch[channelIdIndex]
	customName := submatch[customNameIndex]
	if len(channelId) > 0 {
		call = call.Id(channelId)
	}
	if len(customName) > 0 {
		// TODO: add support for id and for customUrl
		return ChannelInfo{}, ErrCustomUrl
	}
	return executeChannelSearch(call)
}

func (s *Service) FindChannelById(ctx context.Context, id string) (ChannelInfo, error) {
	call := s.yt.Channels.List(snippetPart).Context(ctx).MaxResults(1).Id(id)
	return executeChannelSearch(call)
}

func executeChannelSearch(call *ytApi.ChannelsListCall) (ChannelInfo, error) {
	response, err := call.Do()
	if err != nil {
		return ChannelInfo{}, errors.Wrap(err, "error on calling youtube api")
	}
	items := response.Items
	if len(items) == 0 {
		return ChannelInfo{}, errors.New("unable to find channel")
	}
	if len(items) > 1 {
		fmt.Printf("unexpected item count (%v) during search for channel", len(items))
	}
	channel := items[0]
	if channel.Snippet == nil {
		return ChannelInfo{}, errors.New("snippet is missing in response")
	}
	info := ChannelInfo{
		Id:    channel.Id,
		Title: channel.Snippet.Title,
	}
	return info, nil
}

func (s *Service) GetStreamInfo(videoId string) (StreamInfo, error) {
	part := append(snippetPart, livestreamingDetailsPart...)
	video, err := s.getVideo(videoId, part)
	if err != nil {
		return StreamInfo{}, err
	}
	snippet := video.Snippet
	if snippet == nil {
		return StreamInfo{}, errors.New("snippet is nil")
	}
	streamingDetails := video.LiveStreamingDetails
	if streamingDetails == nil {
		return StreamInfo{}, ErrNotStream
	}
	isUpcoming := false
	startTime := time.Time{}
	broadcastContent := snippet.LiveBroadcastContent
	if broadcastContent != liveEventType && broadcastContent != upcomingEventType {
		return StreamInfo{}, ErrNotStream
	}
	if broadcastContent == upcomingEventType {
		isUpcoming = true
		startTime, err = parseTime(streamingDetails.ScheduledStartTime)
		if err != nil {
			return StreamInfo{}, errors.Errorf("unable to parse scheduled time: %v; source: %v", err.Error(), streamingDetails.ScheduledStartTime)
		}
	}
	return StreamInfo{
		Id: video.Id,
		Channel: ChannelInfo{
			Id:    video.Snippet.ChannelId,
			Title: video.Snippet.ChannelTitle,
		},
		Title:          video.Snippet.Title,
		IsUpcoming:     isUpcoming,
		ScheduledStart: startTime,
	}, nil
}

func (s *Service) getVideo(videoId string, part []string) (*ytApi.Video, error) {
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	defer cancel()
	response, err := s.yt.Videos.
		List(part).
		Context(ctx).
		Id(videoId).
		Do()
	items := response.Items
	itemsCount := len(items)
	if itemsCount == 0 || itemsCount > 1 {
		return nil, errors.Errorf("unexpected number of items: %v", itemsCount)
	}
	return items[0], err
}
