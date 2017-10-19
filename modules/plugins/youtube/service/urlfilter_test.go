package service

import "testing"

func TestGetId(t *testing.T) {
	f := urlfilter{}

	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("youtube.compileRegexpSet() got panic while compiling regular expressions")
		}
	}()

	f.Init()

	id, ok := f.GetId("https://www.youtube.com/watch?v=BMQdZRLi_WM&list=PLywiNEAPE4I9mIv_edkzGeyJkeJmB9b8J")
	if ok == false || id != "BMQdZRLi_WM" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = f.GetId("https://youtu.be/zXPc4Gmj4B8")
	if ok == false || id != "zXPc4Gmj4B8" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = f.GetId("https://www.youtube.com/watch?v=4wjcvhVSEO8&feature=youtu.be")
	if ok == false || id != "4wjcvhVSEO8" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = f.GetId("https://www.youtube.com/channel/UChwOX1m8gxuf_3191ozxqWw")
	if ok == false || id != "UChwOX1m8gxuf_3191ozxqWw" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = f.GetId("https://www.youtube.com/user/abcdefg")
	if ok == false || id != "abcdefg" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}

	id, ok = f.GetId("https://m.youtube.com/channel/abcdefg")
	if ok == false || id != "abcdefg" {
		t.Fatalf("youtube.getIdFromUrl() failed to extract id from valid url")
	}
}
