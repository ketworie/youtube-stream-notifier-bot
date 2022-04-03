package youtube

import (
	"context"
	"fmt"
	ytApi "google.golang.org/api/youtube/v3"
	"time"
)

const (
	searchTimeout = time.Second * 5
	// YouTube API maximum
	searchMaxResults = 50
	// Need to delay youtube poll to prevent exceeding api quota
	pollDelay         = time.Minute
	liveEventType     = "live"
	upcomingEventType = "upcoming"
	videoType         = "video"
)

func (s *Service) PollStreams(channels <-chan ChannelInfo) <-chan StreamInfo {
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
				upcomingBroadcast, err := s.getVideo(item.Id.VideoId, livestreamingDetailsPart)
				if err != nil {
					fmt.Printf("error during search for upcoming stream %v", err.Error())
					continue
				}
				startTimeText := upcomingBroadcast.LiveStreamingDetails.ScheduledStartTime
				startTime, err := parseTime(startTimeText)
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

func parseTime(timeText string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, timeText)
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
		MaxResults(searchMaxResults).
		Do()
	return response, err
}
