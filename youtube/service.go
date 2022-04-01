package youtube

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	ytApi "google.golang.org/api/youtube/v3"
	"os"
	"regexp"
	"strings"
)

// TODO: separate /c and /user to get customUrl or id
var urlPattern = regexp.MustCompile("(https?://)?(www\\.)?youtu((\\.be)|(be\\..{2,5}?))/(channel/(UC[\\w-]{21}[AQgw])|(c/|user/)?([\\w-]+))")

const (
	channelIdIndex  = 7
	customNameIndex = 9
)

var (
	snippetPart              = []string{"snippet"}
	livestreamingDetailsPart = []string{"liveStreamingDetails"}
	recentUploadsSectionType = "recentuploads"
	ErrWrongUrl              = errors.New("unable to parse url")
	ErrCustomUrl             = errors.New("custom url is not supported")
)

type Service struct {
	yt *ytApi.Service
}

func NewService() (*Service, error) {
	service, err := ytApi.NewService(context.Background(), option.WithAPIKey(os.Getenv("YT_API_KEY")))
	if err != nil {
		return nil, err
	}
	return &Service{service}, nil
}

func (s *Service) FindChannel(ctx context.Context, url string) (ChannelInfo, error) {
	submatch := urlPattern.FindStringSubmatch(url)
	if submatch == nil {
		return ChannelInfo{}, ErrWrongUrl
	}
	call := s.yt.Channels.List(snippetPart).Context(ctx).MaxResults(1)
	channelId := submatch[channelIdIndex]
	customName := submatch[customNameIndex]
	if len(channelId) > 0 {
		call = call.Id(channelId)
	}
	if len(customName) > 0 {
		// TODO: add support for id and for customUrl
		return ChannelInfo{}, ErrCustomUrl
	}
	return s.executeChannelSearch(ctx, call)
}

func (s *Service) FindChannelById(ctx context.Context, id string) (ChannelInfo, error) {
	call := s.yt.Channels.List(snippetPart).Context(ctx).MaxResults(1).Id(id)
	return s.executeChannelSearch(ctx, call)
}

func (s *Service) executeChannelSearch(ctx context.Context, call *ytApi.ChannelsListCall) (ChannelInfo, error) {
	response, err := call.Do()
	if err != nil {
		return ChannelInfo{}, errors.Wrap(err, "error on calling youtube api")
	}
	items := response.Items
	if len(items) == 0 {
		return ChannelInfo{}, errors.New("unable to find channel")
	}
	if len(items) > 1 {
		fmt.Printf("unexpected item count (%v) during search for channel", len(items))
	}
	channel := items[0]
	if channel.Snippet == nil {
		return ChannelInfo{}, errors.New("snippet is missing in response")
	}
	parts := append(snippetPart, "id")
	sectionResponse, err := s.yt.ChannelSections.List(parts).ChannelId(channel.Id).Context(ctx).Do()
	if err != nil {
		return ChannelInfo{}, errors.Wrap(err, "error during search for channel sections")
	}
	sections := sectionResponse.Items
	if len(sections) == 0 {
		return ChannelInfo{}, errors.New("channel has no sections")
	}
	var recentUploadsSectionId string
	for _, section := range sections {
		if strings.ToLower(section.Snippet.Type) == recentUploadsSectionType {
			recentUploadsSectionId = section.Id
		}
	}
	if len(recentUploadsSectionId) == 0 {
		return ChannelInfo{}, errors.New("could not find recent uploads section")
	}
	info := ChannelInfo{
		Id:                     channel.Id,
		Title:                  channel.Snippet.Title,
		RecentUploadsSectionId: recentUploadsSectionId,
	}
	return info, nil
}
