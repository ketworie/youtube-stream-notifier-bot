package db

import (
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
	c := Channel{Id: channelId, LeaseSeconds: lease}
	update, err := d.db.Model(&c).Set("lease_seconds = ?lease_seconds").WherePK().Update()
	if err != nil {
		log.Printf("unable to save lease seconds: %v, channelId: %v", err.Error(), channelId)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if update.RowsAffected() == 0 {
		log.Printf("zero rows affected during saving lease seconds: %v, channelId: %v", err.Error(), channelId)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_, err = w.Write([]byte(challenge))
	if update.RowsAffected() == 0 {
		log.Printf("error during write challenge: %v", channelId)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
