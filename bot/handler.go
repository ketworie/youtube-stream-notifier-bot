package bot

import (
	"encoding/xml"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"net/http"
	"youtube-stream-notifier-bot/youtube"
)

func (s *Service) getFeedHandler(streams chan youtube.StreamInfo) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		var feed youtube.Feed
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			log.Printf("unable to read feed body: %v", err.Error())
			return
		}
		err = xml.Unmarshal(body, &feed)
		if err != nil {
			log.Printf("unable to decode incoming feed: %v; source: %v", err.Error(), string(body))
			return
		}
		videoId := feed.Entry.VideoId
		info, err := s.youtube.GetStreamInfo(videoId)
		if err != nil && errors.Is(err, youtube.ErrNotStream) {
			return
		}
		if err != nil {
			log.Printf("unable to get stream info: %v; videoId: %v", err.Error(), videoId)
			return
		}
		streams <- info
	}
}
