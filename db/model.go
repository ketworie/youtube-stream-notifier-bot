package db

import "time"

type Chat struct {
	Id      int64
	Enabled bool
}

type Channel struct {
	Id         string
	Title      string
	LastUpdate time.Time
}

type Subscription struct {
	Id        int64
	ChatId    int64
	ChannelId string
}

type DoneStream struct {
	Id           string
	DoneUpcoming bool
	DoneLive     bool
}
