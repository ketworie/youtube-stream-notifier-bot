package timezone

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"log"
	"net/http"
	"net/url"
)

type Service struct {
	token string
}

func NewService(token string) *Service {
	return &Service{token: token}
}

const getTZURL = "http://api.timezonedb.com/v2.1/get-time-zone"

func (s *Service) GetTimeZone(lat, lng string) (string, error) {
	values := url.Values{}
	values.Set("key", s.token)
	values.Set("format", "json")
	values.Set("by", "position")
	values.Set("fields", "zoneName")
	values.Set("lat", lat)
	values.Set("lng", lng)
	response, err := http.Get(fmt.Sprintf("%v?%v", getTZURL, values.Encode()))
	if err != nil {
		return "", errors.Wrap(err, "unable to get timezone from timezonedb")
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Printf("Error during closing TZ body: %v", err.Error())
		}
	}()
	tzPayload := struct {
		ZoneName string `json:"zoneName"`
	}{}
	err = json.NewDecoder(response.Body).Decode(&tzPayload)
	if err != nil {
		return "", errors.Wrap(err, "unable to decode timezone from timezonedb")
	}
	return tzPayload.ZoneName, nil
}
