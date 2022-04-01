package db

import (
	"context"
	"fmt"
	"time"
)

func (d *DB) PollChannels(ctx context.Context) <-chan Channel {
	ids := make(chan Channel)
	go func() {
		defer close(ids)
		d.startPolling(ctx, ids)
	}()
	return ids
}

func (d *DB) startPolling(ctx context.Context, ids chan Channel) {
	for {
		channels, err := d.ListActiveChannels()
		if err != nil {
			fmt.Println(err.Error())
			time.Sleep(time.Second * 10)
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
