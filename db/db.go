package db

import (
	"context"
	"database/sql"
	"github.com/go-pg/pg/v10"
	"github.com/pkg/errors"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	"log"
	"time"
)

var (
	ErrNotFound = errors.New("entity not found")
)

type DB struct {
	db      *bun.DB
	timeout time.Duration
}

const defaultTimeout = time.Minute

func New(address, user, password, database string) *DB {
	connector := pgdriver.NewConnector(
		pgdriver.WithInsecure(true),
		pgdriver.WithAddr(address),
		pgdriver.WithUser(user),
		pgdriver.WithPassword(password),
		pgdriver.WithDatabase(database),
	)
	sqldb := sql.OpenDB(connector)
	db := bun.NewDB(sqldb, pgdialect.New())
	return &DB{db: db, timeout: defaultTimeout}
}

func (d *DB) SetTimeout(duration time.Duration) {
	d.timeout = duration
}

func (d *DB) EnableDebug() {
	d.db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
}

func (d *DB) GetChat(id int64) (Chat, error) {
	u := Chat{Id: id}
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	err := d.db.NewSelect().Model(&u).WherePK().Scan(ctx)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return Chat{}, ErrNotFound
	}
	if err != nil {
		return Chat{}, errors.Wrap(err, "error during querying user")
	}
	return u, nil
}

func (d *DB) AddChat(u Chat) error {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	_, err := d.db.NewInsert().Model(&u).Exec(ctx)
	if err != nil {
		return errors.Wrap(err, "error during adding user")
	}
	return nil
}

func (d *DB) SetChatTimeZone(id int64, timeZone string) error {
	c := Chat{
		Id:       id,
		TimeZone: &timeZone,
	}
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	_, err := d.db.NewUpdate().Model(&c).Set("time_zone = ?time_zone").WherePK().Exec(ctx)
	return err
}

func (d *DB) GetChannel(id string) (Channel, error) {
	c := Channel{Id: id}
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	err := d.db.NewSelect().Model(&c).WherePK().Scan(ctx)
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
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	return d.db.NewSelect().Model(&c).WherePK().Exists(ctx)
}

func (d *DB) AddChannel(c Channel) error {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	_, err := d.db.NewInsert().Model(&c).Exec(ctx)
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
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	exists, err := d.db.
		NewSelect().
		Model(&sub).
		Where("chat_id = ?", sub.ChatId).
		Where("channel_id = ?", sub.ChannelId).
		Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	ctx, cancel = context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	_, err = d.db.NewInsert().Model(&sub).Exec(ctx)
	return err
}

func (d *DB) GetSubscribedChannels(chatId int64) ([]Channel, error) {
	var channels []Channel
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	err := d.db.NewSelect().
		Model(&channels).
		Join("LEFT JOIN subscriptions AS s ON s.channel_id = channel.id").
		Where("s.chat_id = ?", chatId).
		Scan(ctx)
	return channels, err
}

func (d *DB) GetSubscribedChats(channelId string) ([]Chat, error) {
	var chats []Chat
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	err := d.db.NewSelect().
		Model(&chats).
		Join("LEFT JOIN subscriptions AS s ON s.chat_id = chat.id").
		Where("s.channel_id = ?", channelId).
		Where("chat.enabled = ?", true).
		Scan(ctx)
	return chats, err
}

func (d *DB) RemoveSubscription(chatId int64, channelId string) error {
	sub := Subscription{ChatId: chatId, ChannelId: channelId}
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	_, err := d.db.NewDelete().
		Model(&sub).
		Where("chat_id = ?", chatId).
		Where("channel_id = ?", channelId).
		Exec(ctx)
	return err
}

func (d *DB) ListActiveChannels() ([]Channel, error) {
	var channels []Channel
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	err := d.db.NewSelect().
		Model(&channels).
		Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.channel_id = channel.id)").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (d *DB) ListLeaseExpiringChannels() ([]Channel, error) {
	var channels []Channel
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	err := d.db.NewSelect().
		Model(&channels).
		Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.channel_id = channel.id)").
		Where(
			"(last_update + (channel.lease_seconds || ' seconds')::interval) < (NOW() + interval '5 minutes') " +
				"OR channel.lease_seconds IS NULL",
		).
		Scan(ctx)
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
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	exists, err := d.db.NewSelect().Model(&ds).WherePK().Exists(ctx)
	if err != nil {
		log.Printf("Unable to check if DoneStream with id %v exists: %v", ds.Id, err.Error())
	}
	if exists {
		ctx, cancel = context.WithTimeout(context.Background(), d.timeout)
		defer cancel()
		_, err := d.db.NewUpdate().Model(&ds).WherePK().Exec(ctx)
		return err
	}
	ctx, cancel = context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	_, err = d.db.NewInsert().Model(&ds).Exec(ctx)
	return err
}

func (d *DB) IsUpcomingDone(streamId string) (bool, error) {
	ds := DoneStream{Id: streamId}
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	return d.db.NewSelect().
		Model(&ds).
		WherePK().
		Where("done_upcoming = ?", true).
		Exists(ctx)
}

func (d *DB) IsLiveDone(streamId string) (bool, error) {
	ds := DoneStream{Id: streamId}
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	return d.db.NewSelect().
		Model(&ds).
		WherePK().
		Where("done_live = ?", true).
		Exists(ctx)
}
