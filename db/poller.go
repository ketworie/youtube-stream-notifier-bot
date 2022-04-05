package db

import (
	"context"
	"fmt"
	"time"
)

const (
	sleepOnErrorTime = time.Second * 10
	// Prevent overload
	sleepOnEmptyTime = time.Minute
	// Need to have minimal interval to get time for subscription confirmation
	minimumLeaseExpiringPollInterval = time.Minute
)

func (d *DB) PollChannels(ctx context.Context, leaseExpiring bool) <-chan Channel {
	ids := make(chan Channel)
	go func() {
		defer close(ids)
		d.startPolling(ctx, ids, leaseExpiring)
	}()
	return ids
}

func (d *DB) startPolling(ctx context.Context, ids chan Channel, leaseExpiring bool) {
	for {
		then := time.Now()
		var channels []Channel
		var err error
		if leaseExpiring {
			channels, err = d.ListLeaseExpiringChannels()
		} else {
			channels, err = d.ListActiveChannels()
		}
		if err != nil {
			fmt.Println(err.Error())
			time.Sleep(sleepOnErrorTime)
			continue
		}
		for _, channel := range channels {
			select {
			case <-ctx.Done():
				return
			default:
				ids <- channel
			}
		}
		if len(channels) == 0 {
			time.Sleep(sleepOnEmptyTime)
		}
		delta := minimumLeaseExpiringPollInterval.Nanoseconds() - time.Now().Sub(then).Nanoseconds()
		if leaseExpiring && delta > 0 {
			time.Sleep(time.Duration(delta))
		}
	}
}
