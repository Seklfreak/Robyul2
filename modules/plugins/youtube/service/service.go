package service

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/Seklfreak/Robyul2/helpers"
	"golang.org/x/oauth2/google"
	youtubeAPI "google.golang.org/api/youtube/v3"
)

type Service struct {
	service *youtubeAPI.Service
	quota   quota
	filter  *urlfilter

	sync.RWMutex
}

func (s *Service) Init(configFilePath string) {
	s.Lock()
	defer s.Unlock()

	if s.service != nil {
		s.stop()
	}

	s.init(configFilePath)

	err := s.quota.Init()
	helpers.Relax(err)

	s.filter = newUrlFilter()
}

func (s *Service) Stop() {
	s.Lock()
	defer s.Unlock()

	s.stop()
}

// searchQuerySingle returns single search result with given type @searchType.
// returns (nil, nil) when there is no matching results.
func (s *Service) SearchQuerySingle(keywords []string, searchType string) (*youtubeAPI.SearchResult, error) {
	s.RLock()
	defer s.RUnlock()

	if s.service == nil {
		return nil, errors.New("plugins.youtubs.service-not-available")
	}

	s.quota.Sub(searchQuotaCost)

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
		return nil, s.handleGoogleAPIError(err)
	}

	if len(response.Items) < 1 {
		return nil, nil
	}

	return response.Items[0], nil
}

func (s *Service) GetChannelFeeds(channelId, publishedAfter string) ([]*youtubeAPI.Activity, error) {
	s.RLock()
	defer s.RUnlock()

	if s.service == nil {
		return nil, errors.New("plugins.youtubs.service-not-available")
	}

	s.quota.Sub(activityQuotaCost)

	call := s.service.Activities.List("contentDetails, snippet").
		ChannelId(channelId).
		PublishedAfter(publishedAfter).
		MaxResults(50)

	response, err := call.Do()
	if err != nil {
		return nil, s.handleGoogleAPIError(err)
	}

	return response.Items, nil
}

func (s *Service) GetVideoSingle(videoId string) (*youtubeAPI.Video, error) {
	s.RLock()
	defer s.RUnlock()

	if s.service == nil {
		return nil, errors.New("plugins.youtubs.service-not-available")
	}

	s.quota.Sub(videosQuotaCost)

	call := s.service.Videos.List("statistics, snippet").
		Id(videoId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return nil, s.handleGoogleAPIError(err)
	}

	if len(response.Items) < 1 {
		return nil, nil
	}

	return response.Items[0], nil
}

func (s *Service) GetChannelSingle(channelId string) (*youtubeAPI.Channel, error) {
	s.RLock()
	defer s.RUnlock()

	if s.service == nil {
		return nil, errors.New("plugins.youtubs.service-not-available")
	}

	s.quota.Sub(channelsQuotaCost)

	call := s.service.Channels.List("statistics, snippet").
		Id(channelId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return nil, s.handleGoogleAPIError(err)
	}

	if len(response.Items) < 1 {
		return nil, nil
	}

	return response.Items[0], nil
}

func (s *Service) IncQuotaEntryCount() {
	s.quota.IncEntryCount()
}

func (s *Service) DecQuotaEntryCount() {
	s.quota.DecEntryCount()
}

func (s *Service) UpdateCheckingInterval() error {
	return s.quota.UpdateCheckingInterval()
}

func (s *Service) GetCheckingInterval() int64 {
	return s.quota.GetInterval()
}

func (s *Service) init(configFilePath string) {
	configFile := helpers.GetConfig().Path(configFilePath).Data().(string)

	authJSON, err := ioutil.ReadFile(configFile)
	helpers.Relax(err)

	config, err := google.JWTConfigFromJSON(authJSON, youtubeAPI.YoutubeReadonlyScope)
	helpers.Relax(err)

	client := config.Client(context.Background())

	s.service, err = youtubeAPI.New(client)
	helpers.Relax(err)
}

func (s *Service) stop() {
	s.service = nil
}

func (s *Service) handleGoogleAPIError(err error) error {
	var errCode int
	var errMsg string
	_, scanErr := fmt.Sscanf(err.Error(), "googleapi: Error %d: %s", &errCode, &errMsg)
	if scanErr != nil {
		return err
	}

	// Handle google API error by code
	switch errCode {
	case 403:
		s.quota.DailyLimitExceeded()
		return fmt.Errorf("plugins.youtube.daily-limit-exceeded")
	default:
		return err
	}
}
