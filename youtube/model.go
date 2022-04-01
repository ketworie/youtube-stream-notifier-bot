package youtube

import "time"

type ChannelHeader struct {
	Id    string
	Title string
}

type ChannelInfo struct {
	Id                     string
	Title                  string
	RecentUploadsSectionId string
}

type StreamInfo struct {
	Id             string
	Channel        ChannelHeader
	Title          string
	IsUpcoming     bool
	ScheduledStart time.Time
}
