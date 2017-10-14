package youtube

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/Seklfreak/Robyul2/helpers"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

type service struct {
	service *youtube.Service
	filter  urlfilter

	sync.RWMutex
}

func (s *service) Init(configFilePath string) {
	s.Lock()
	defer s.Unlock()

	if s.service != nil {
		s.stop()
	}

	s.filter.Init()
	s.init(configFilePath)
}

func (s *service) Stop() {
	s.Lock()
	defer s.Unlock()

	s.stop()
}

// searchQuerySingle retuns single search result with given type @searchType.
// returns (nil, nil) when there is no matching results.
func (s *service) SearchQuerySingle(keywords []string, searchType string) (*youtube.SearchResult, error) {
	s.RLock()
	defer s.RUnlock()

	if s.service == nil {
		return nil, errors.New("plugins.youtube.service-not-available")
	}

	// extract ID from valid youtube url
	for i, w := range keywords {
		keywords[i], _ = s.filter.GetId(w)
	}

	query := strings.Join(keywords, " ")

	call := s.service.Search.List("id, snippet").
		Type(searchType).
		MaxResults(1).
		Q(query)

	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) < 1 {
		return nil, nil
	}

	return response.Items[0], nil
}

func (s *service) GetChannelFeeds(channelId, publishedAfter string) ([]*youtube.Activity, error) {
	s.RLock()
	defer s.RUnlock()

	if s.service == nil {
		return nil, errors.New("plugins.youtube.service-not-available")
	}

	call := s.service.Activities.List("contentDetails, snippet").
		ChannelId(channelId).
		PublishedAfter(publishedAfter).
		MaxResults(50)

	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	return response.Items, nil
}

func (s *service) GetVideoSingle(videoId string) (*youtube.Video, error) {
	s.RLock()
	defer s.RUnlock()

	if s.service == nil {
		return nil, errors.New("plugins.youtube.service-not-available")
	}

	call := s.service.Videos.List("statistics, snippet").
		Id(videoId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) < 1 {
		return nil, nil
	}

	return response.Items[0], nil
}

func (s *service) GetChannelSingle(channelId string) (*youtube.Channel, error) {
	s.RLock()
	defer s.RUnlock()

	if s.service == nil {
		return nil, errors.New("plugins.youtube.service-not-available")
	}

	call := s.service.Channels.List("statistics, snippet").
		Id(channelId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) < 1 {
		return nil, nil
	}

	return response.Items[0], nil
}

func (s *service) init(configFilePath string) {
	configFile := helpers.GetConfig().Path(configFilePath).Data().(string)

	authJSON, err := ioutil.ReadFile(configFile)
	helpers.Relax(err)

	config, err := google.JWTConfigFromJSON(authJSON, youtube.YoutubeReadonlyScope)
	helpers.Relax(err)

	client := config.Client(context.Background())

	s.service, err = youtube.New(client)
	helpers.Relax(err)
}

func (s *service) stop() {
	s.service = nil
}
