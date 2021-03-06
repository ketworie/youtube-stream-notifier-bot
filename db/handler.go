package db

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"youtube-stream-notifier-bot/youtube"
)

func (d *DB) HandleConfirmSubscription(w http.ResponseWriter, r *http.Request) {
	mode := r.FormValue(youtube.HubMode)
	topic := r.FormValue(youtube.HubTopic)
	challenge := r.FormValue(youtube.HubChallenge)
	leaseSeconds := r.FormValue(youtube.HubLeaseSeconds)
	if mode != youtube.HubModeSubscribe {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	submatch := youtube.HubTopicPattern.FindStringSubmatch(topic)
	if submatch == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	channelId := submatch[1]
	lease, err := strconv.Atoi(leaseSeconds)
	if err != nil {
		log.Printf("unable to parse lease seconds: %v, source: %v", err.Error(), leaseSeconds)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	c := Channel{Id: channelId, LeaseSeconds: &lease}
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	update, err := d.db.NewUpdate().Model(&c).Set("lease_seconds = ?lease_seconds").WherePK().Exec(ctx)
	if err != nil {
		log.Printf("unable to save lease seconds: %v, channelId: %v", err.Error(), channelId)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	rowsAffected, err := update.RowsAffected()
	if err != nil {
		log.Printf("error during saving lease seconds: %v", err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if rowsAffected == 0 {
		log.Printf("zero rows affected during saving lease seconds: %v, channelId: %v", lease, channelId)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_, err = w.Write([]byte(challenge))
	if err != nil {
		log.Printf("error during write challenge: %v", channelId)
		return
	}
}
