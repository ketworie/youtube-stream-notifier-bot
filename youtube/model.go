package youtube

import "time"

type ChannelInfo struct {
	Id    string
	Title string
}

type StreamInfo struct {
	Id             string
	Channel        ChannelInfo
	Title          string
	IsUpcoming     bool
	ScheduledStart time.Time
}
