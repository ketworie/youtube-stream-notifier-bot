package mutex

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis"
	"time"
)

const (
	streamLockExpiration     = time.Minute * 5
	streamChatLockExpiration = time.Minute * 5
	streamKeyPattern         = "stream:%v"
	streamChatKeyPattern     = "stream:%v:user:%v"
)

type Builder struct {
	rs *redsync.Redsync
}

func NewBuilder(address string) *Builder {
	client := redis.NewClient(&redis.Options{Addr: address})
	pool := goredis.NewPool(client)
	rs := redsync.New(pool)
	return &Builder{rs: rs}
}

func (c *Builder) Stream(streamId string) *redsync.Mutex {
	key := fmt.Sprintf(streamKeyPattern, streamId)
	mutex := c.rs.NewMutex(key, redsync.WithExpiry(streamLockExpiration))
	return mutex
}

func (c *Builder) LockStreamChat(streamId string, chatId int64) *redsync.Mutex {
	key := fmt.Sprintf(streamChatKeyPattern, streamId, chatId)
	return c.rs.NewMutex(key, redsync.WithExpiry(streamChatLockExpiration))
}
