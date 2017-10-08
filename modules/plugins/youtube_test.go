package plugins

import "testing"
import "github.com/bwmarrin/discordgo"

func TestYoutubeRegexp(t *testing.T) {
	testYoutube := YouTube{}

	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("youtube.compileRegexpSet() got panic while compiling regular expressions")
		}
	}()

	testYoutube.compileRegexpSet(videoLongUrl, videoShortUrl, channelIdUrl, channelUserUrl)

	id, ok := testYoutube.getIdFromUrl("https://www.youtube.com/watch?v=BMQdZRLi_WM&list=PLywiNEAPE4I9mIv_edkzGeyJkeJmB9b8J")
	if ok == false || id != "BMQdZRLi_WM" {
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

	id, ok = testYoutube.getIdFromUrl("https://m.youtube.com/channel/abcdefg")
	if ok == false || id != "abcdefg" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}
}

func TestVerifyEmbedFields(t *testing.T) {
	testYoutube := YouTube{}

	fields := []*discordgo.MessageEmbedField{
		{Name: "", Value: "value"},
		{Name: "name", Value: ""},
		{Name: "name", Value: "0"},
	}

	fields = testYoutube.verifyEmbedFields(fields)
	if len(fields) != 0 {
		t.Fatalf("youtube.verifyEmbedFields() failed to trim invalid field")
	}
}

func TestGetChannelContent(t *testing.T) {
	e := DB_Youtube_Entry{}

	// Wrong content type checking
	e.ContentType = "wrong type"
	_, err := e.getChannelContent()
	if err == nil {
		t.Fatalf("DB_Youtube_Entry.getChannelContent() failed to ContentType string checking")
	}
	e.ContentType = "channel"

	e.Content = "string"
	_, err = e.getChannelContent()
	if err == nil {
		t.Fatalf("DB_Youtube_Entry.getChannelContent() allows not map[string]interface{} type of Content")
	}

	// Empty fields checking
	c := make(map[string]interface{})
	e.Content = c
	_, err = e.getChannelContent()
	if err == nil {
		t.Fatalf("DB_Youtube_Entry.getChannelContent() allows empty fields")
	}

	// Wrong content fields type checking
	c["content_channel_id"] = 100
	c["content_channel_name"] = 100
	c["content_channel_posted_videos"] = 100
	e.Content = c

	_, err = e.getChannelContent()
	if err == nil {
		t.Fatalf("DB_Youtube_Entry.getChannelContent() allows wrong type of content fields")
	}
}
