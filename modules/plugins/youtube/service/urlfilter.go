package service

import "regexp"

type urlfilter struct {
	regexpSet []*regexp.Regexp
}

const (
	videoLongUrl   string = `^(https?\:\/\/)?(www\.|m\.)?(youtube\.com)\/watch\?v=(.[A-Za-z0-9_]*)`
	videoShortUrl  string = `^(https?\:\/\/)?(youtu\.be)\/(.[A-Za-z0-9_]*)`
	channelIdUrl   string = `^(https?\:\/\/)?(www\.|m\.)?(youtube\.com)\/channel\/(.[A-Za-z0-9_]*)`
	channelUserUrl string = `^(https?\:\/\/)?(www\.|m\.)?(youtube\.com)\/user\/(.[A-Za-z0-9_]*)`
)

func (f *urlfilter) Init() {
	f.compileRegexpSet(videoLongUrl, videoShortUrl, channelIdUrl, channelUserUrl)
}

// GetId extracts channel id, channel name, video id from given url.
func (f *urlfilter) GetId(url string) (id string, ok bool) {
	// TODO: it failed to retrieve exact information from user name.
	// example) https://www.youtube.com/user/bruno
	for i := range f.regexpSet {
		if f.regexpSet[i].MatchString(url) {
			match := f.regexpSet[i].FindStringSubmatch(url)
			return match[len(match)-1], true
		}
	}

	return url, false
}

func (f *urlfilter) compileRegexpSet(regexps ...string) {
	for i := range f.regexpSet {
		f.regexpSet[i] = nil
	}
	f.regexpSet = f.regexpSet[:0]

	for i := range regexps {
		f.regexpSet = append(f.regexpSet, regexp.MustCompile(regexps[i]))
	}
}
