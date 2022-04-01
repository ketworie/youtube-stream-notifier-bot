package youtube

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	ytApi "google.golang.org/api/youtube/v3"
	"time"
)

const (
	searchTimeout = time.Second * 5
	// Need to delay youtube poll to prevent exceeding api quota
	pollDelay         = time.Minute
	liveEventType     = "live"
	upcomingEventType = "upcoming"
	videoType         = "video"
)

func (s *Service) PollStreams(channels <-chan ChannelHeader) <-chan StreamInfo {
	streams := make(chan StreamInfo)
	go func() {
		defer close(streams)
		for channel := range channels {
			response, err := s.searchVideos(channel.Id, liveEventType)
			if err != nil {
				fmt.Printf("error during search for live streams %v", err.Error())
				continue
			}
			for _, item := range response.Items {
				streams <- StreamInfo{
					Id:         item.Id.VideoId,
					Channel:    channel,
					Title:      item.Snippet.Title,
					IsUpcoming: false,
				}
			}
			response, err = s.searchVideos(channel.Id, upcomingEventType)
			if err != nil {
				fmt.Printf("error during search for upcoming streams %v", err.Error())
				continue
			}
			for _, item := range response.Items {
				upcomingBroadcast, err := s.GetUpcomingBroadcast(item.Id.VideoId)
				if err != nil {
					fmt.Printf("error during search for upcoming stream %v", err.Error())
					continue
				}
				startTimeText := upcomingBroadcast.LiveStreamingDetails.ScheduledStartTime
				startTime, err := time.Parse(time.RFC3339Nano, startTimeText)
				if err != nil {
					fmt.Printf("unable to parse time %v during search for upcoming stream", startTimeText)
					continue
				}
				streams <- StreamInfo{
					Id:             item.Id.VideoId,
					Channel:        channel,
					Title:          item.Snippet.Title,
					IsUpcoming:     true,
					ScheduledStart: startTime,
				}
			}
			time.Sleep(pollDelay)
		}
	}()
	return streams
}

func (s *Service) GetUpcomingBroadcast(videoId string) (*ytApi.Video, error) {
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	defer cancel()
	response, err := s.yt.Videos.
		List(livestreamingDetailsPart).
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

func (s *Service) searchVideos(channelId string, eventType string) (*ytApi.SearchListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	defer cancel()
	response, err := s.yt.Search.
		List(snippetPart).
		Context(ctx).
		ChannelId(channelId).
		EventType(eventType).
		Type(videoType).
		Do()
	return response, err
}
