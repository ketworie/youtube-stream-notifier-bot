package youtube

import "regexp"

const (
	HubMode         = "hub.mode"
	HubTopic        = "hub.topic"
	HubChallenge    = "hub.challenge"
	HubLeaseSeconds = "hub.lease_seconds"
	HubCallback     = "hub.callback"
	HubVerify       = "hub.verify"
)

const (
	HubModeSubscribe          = "subscribe"
	HubVerifyAsync            = "async"
	HubTopicFormat            = "https://www.youtube.com/xml/feeds/videos.xml?channel_id=%v"
	HubYouTubeURL             = "https://pubsubhubbub.appspot.com/subscribe"
	HubSubscribePathURLFormat = "http://%v/video"
)

var (
	HubTopicPattern = regexp.MustCompile("https://www\\.youtube\\.com/xml/feeds/videos\\.xml\\?channel_id=(.+)")
)
