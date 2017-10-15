package youtube

import "testing"
import "github.com/bwmarrin/discordgo"

func TestVerifyEmbedFields(t *testing.T) {
	fields := []*discordgo.MessageEmbedField{
		{Name: "", Value: "value"},
		{Name: "name", Value: ""},
		{Name: "name", Value: "0"},
	}

	fields = verifyEmbedFields(fields)
	if len(fields) != 0 {
		t.Fatalf("youtube.verifyEmbedFields() failed to trim invalid field")
	}
}
