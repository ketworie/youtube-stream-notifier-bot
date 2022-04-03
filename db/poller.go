package db

import (
	"context"
	"fmt"
	"time"
)

const sleepOnErrorTime = time.Second * 10

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
	}
}
