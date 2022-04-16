package db

import "time"

type Chat struct {
	Id       int64 `bun:",pk"`
	TimeZone *string
	Enabled  bool
}

type Channel struct {
	Id           string `bun:",pk"`
	Title        string
	LeaseSeconds *int
	LastUpdate   time.Time
}

type Subscription struct {
	Id        int64 `bun:",pk,autoincrement"`
	ChatId    int64
	ChannelId string
}

type DoneStream struct {
	Id           string `bun:",pk"`
	DoneUpcoming bool
	DoneLive     bool
}
