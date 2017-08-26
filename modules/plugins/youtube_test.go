package plugins

import "testing"

func TestYoutubeRegexp(t *testing.T) {
	testYoutube := YouTube{}

	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("youtube.compileRegexpSet() got panic while compiling regular expressions")
		}
	}()

	testYoutube.compileRegexpSet(videoLongUrl, videoShortUrl, channelIdUrl, channelUserUrl)

	id, ok := testYoutube.getIdFromUrl("https://www.youtube.com/watch?v=zrNxJJ7fxCk")
	if ok == false || id != "zrNxJJ7fxCk" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = testYoutube.getIdFromUrl("https://youtu.be/zXPc4Gmj4B8")
	if ok == false || id != "zXPc4Gmj4B8" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = testYoutube.getIdFromUrl("https://www.youtube.com/watch?v=4wjcvhVSEO8&feature=youtu.be")
	if ok == false || id != "4wjcvhVSEO8" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = testYoutube.getIdFromUrl("https://www.youtube.com/channel/UChwOX1m8gxuf_3191ozxqWw")
	if ok == false || id != "UChwOX1m8gxuf_3191ozxqWw" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = testYoutube.getIdFromUrl("https://www.youtube.com/user/abcdefg")
	if ok == false || id != "abcdefg" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}
}
