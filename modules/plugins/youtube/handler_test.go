package youtube

import "testing"
import "github.com/bwmarrin/discordgo"

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
