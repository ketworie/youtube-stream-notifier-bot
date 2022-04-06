package db

import (
	"context"
	"github.com/go-pg/pg/extra/pgdebug"
	"github.com/go-pg/pg/v10"
	"github.com/pkg/errors"
)

var (
	ErrNotFound = errors.New("entity not found")
)

type DB struct {
	db *pg.DB
}

func New(address, user, password, database string) (*DB, error) {
	db := pg.Connect(&pg.Options{
		Addr:     address,
		User:     user,
		Password: password,
		Database: database,
	})
	if err := db.Ping(context.Background()); err != nil {
		return nil, errors.Wrap(err, "cannot connect to db")
	}
	return &DB{db: db}, nil
}

func (d *DB) EnableDebug() {
	d.db.AddQueryHook(pgdebug.DebugHook{Verbose: true})
}

func (d *DB) GetChat(id int64) (Chat, error) {
	u := Chat{Id: id}
	err := d.db.Model(&u).WherePK().Select()
	if err != nil && errors.Is(err, pg.ErrNoRows) {
		return Chat{}, ErrNotFound
	}
	if err != nil {
		return Chat{}, errors.Wrap(err, "error during querying user")
	}
	return u, nil
}

func (d *DB) AddChat(u Chat) error {
	_, err := d.db.Model(&u).Insert()
	if err != nil {
		return errors.Wrap(err, "error during adding user")
	}
	return nil
}

func (d *DB) GetChannel(id string) (Channel, error) {
	c := Channel{Id: id}
	err := d.db.Model(&c).WherePK().Select()
	if err != nil && errors.Is(err, pg.ErrNoRows) {
		return Channel{}, ErrNotFound
	}
	if err != nil {
		return Channel{}, errors.Wrap(err, "error during querying channel")
	}
	return c, nil
}

func (d *DB) ChannelExists(id string) (bool, error) {
	c := Channel{Id: id}
	return d.db.Model(&c).WherePK().Exists()
}

func (d *DB) AddChannel(c Channel) error {
	_, err := d.db.Model(&c).Insert()
	if err != nil {
		return errors.Wrap(err, "error during adding channel")
	}
	return nil
}

func (d *DB) AddSubscription(userId int64, channelId string) error {
	sub := Subscription{
		ChatId:    userId,
		ChannelId: channelId,
	}
	exists, err := d.db.
		Model(&sub).
		Where("chat_id = ?", sub.ChatId).
		Where("channel_id = ?", sub.ChannelId).
		Exists()
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = d.db.Model(&sub).Insert()
	return err
}

func (d *DB) GetSubscribedChannels(chatId int64) ([]Channel, error) {
	var channels []Channel
	err := d.db.Model(&channels).
		Join("LEFT JOIN subscriptions AS s ON s.channel_id = channel.id").
		Where("s.chat_id = ?", chatId).
		Select()
	return channels, err
}

func (d *DB) GetSubscribedChats(channelId string) ([]Chat, error) {
	var chats []Chat
	err := d.db.Model(&chats).
		Join("LEFT JOIN subscriptions AS s ON s.chat_id = chat.id").
		Where("s.channel_id = ?", channelId).
		Where("chat.enabled = ?", true).
		Select()
	return chats, err
}

func (d *DB) RemoveSubscription(chatId int64, channelId string) error {
	sub := Subscription{ChatId: chatId, ChannelId: channelId}
	_, err := d.db.Model(&sub).
		Where("chat_id = ?", chatId).
		Where("channel_id = ?", channelId).
		Delete()
	return err
}

func (d *DB) ListActiveChannels() ([]Channel, error) {
	var channels []Channel
	err := d.db.Model(&channels).
		Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.channel_id = channel.id)").
		Select()
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (d *DB) ListLeaseExpiringChannels() ([]Channel, error) {
	var channels []Channel
	err := d.db.Model(&channels).
		Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.channel_id = channel.id)").
		Where("(last_update + (channel.lease_seconds || ' seconds')::interval) < (NOW() + interval '5 minutes') " +
			"OR channel.lease_seconds IS NULL").
		Select()
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (d *DB) MarkDone(streamId string, isUpcoming bool) error {
	ds := DoneStream{
		Id:           streamId,
		DoneUpcoming: true,
		DoneLive:     !isUpcoming,
	}
	_, err := d.db.Model(&ds).Insert()
	return err
}

func (d *DB) IsUpcomingDone(streamId string) (bool, error) {
	ds := DoneStream{Id: streamId}
	return d.db.Model(&ds).
		WherePK().
		Where("done_upcoming = ?", true).
		Exists()
}

func (d *DB) IsLiveDone(streamId string) (bool, error) {
	ds := DoneStream{Id: streamId}
	return d.db.Model(&ds).
		WherePK().
		Where("done_live = ?", true).
		Exists()
}
