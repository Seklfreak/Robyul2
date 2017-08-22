// Except.go: Contains functions to make handling panics less PITA

package helpers

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"

	"math/rand"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/getsentry/raven-go"
)

// RecoverDiscord recover()s and sends a message to discord
func RecoverDiscord(msg *discordgo.Message) {
	err := recover()
	if err != nil {
		SendError(msg, err)
	}
}

// Recover recover()s and prints the error to console
func Recover() {
	err := recover()
	if err != nil {
		fmt.Printf("%#v\n", err)

		//raven.SetUserContext(&raven.User{})
		raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
	}
}

// SoftRelax is a softer form of Relax()
// Calls a callback instead of panicking
func SoftRelax(err error, cb Callback) {
	if err != nil {
		cb()
	}
}

// Relax is a helper to reduce if-checks if panicking is allowed
// If $err is nil this is a no-op. Panics otherwise.
func Relax(err error) {
	if err != nil {
		if DEBUG_MODE == true {
			fmt.Printf("%#v\n", err)
			panic(err)
			if err, ok := err.(*discordgo.RESTError); ok && err != nil && err.Message != nil {
				fmt.Println(strconv.Itoa(err.Message.Code)+":", err.Message.Message)
			} else {
				fmt.Println(err)
			}
		}
		panic(err)
	}
}

// RelaxEmbed does nothing if $err is nil, prints a notice if there are no permissions to embed, else sends it to Relax()
func RelaxEmbed(err error, channelID string, commandMessageID string) {
	if err != nil {
		if errD, ok := err.(*discordgo.RESTError); ok {
			if errD.Message.Code == 50013 {
				if channelID != "" {
					_, err = cache.GetSession().ChannelMessageSend(channelID, GetText("bot.errors.no-embed"))
					RelaxMessage(err, channelID, commandMessageID)
				}
				return
			}
		}
		Relax(err)
	}
}

// RelaxEmbed does nothing if $err is nil or if there are no permissions to send a message, else sends it to Relax()
func RelaxMessage(err error, channelID string, commandMessageID string) {
	if err != nil {
		if errD, ok := err.(*discordgo.RESTError); ok {
			if errD.Message.Code == 50013 {
				if channelID != "" && commandMessageID != "" {
					reactions := []string{
						":blobstop:317034621953114112",
						":blobweary:317036265071575050",
						":googlespeaknoevil:317036753074651139",
						":notlikeblob:349342777978519562",
					}
					cache.GetSession().MessageReactionAdd(channelID, commandMessageID, reactions[rand.Intn(len(reactions))])
				}
			} else {
				Relax(err)
			}
		} else {
			Relax(err)
		}
	}
}

// RelaxAssertEqual panics if a is not b
func RelaxAssertEqual(a interface{}, b interface{}, err error) {
	if !reflect.DeepEqual(a, b) {
		Relax(err)
	}
}

// RelaxAssertUnequal panics if a is b
func RelaxAssertUnequal(a interface{}, b interface{}, err error) {
	if reflect.DeepEqual(a, b) {
		Relax(err)
	}
}

// SendError Takes an error and sends it to discord and sentry.io
func SendError(msg *discordgo.Message, err interface{}) {
	if DEBUG_MODE == true {
		buf := make([]byte, 1<<16)
		stackSize := runtime.Stack(buf, false)

		cache.GetSession().ChannelMessageSend(
			msg.ChannelID,
			"Error <:blobfrowningbig:317028438693117962>\n```\n"+fmt.Sprintf("%#v\n", err)+fmt.Sprintf("%s\n", string(buf[0:stackSize]))+"\n```",
		)
	} else {
		if errR, ok := err.(*discordgo.RESTError); ok && errR != nil && errR.Message != nil {
			if msg != nil {
				cache.GetSession().ChannelMessageSend(
					msg.ChannelID,
					"Error <:blobfrowningbig:317028438693117962>\n```\n"+fmt.Sprintf("%#v", errR.Message.Message)+"\n```",
				)
			}
		} else {
			if msg != nil {
				cache.GetSession().ChannelMessageSend(
					msg.ChannelID,
					"Error <:blobfrowningbig:317028438693117962>\n```\n"+fmt.Sprintf("%#v", err)+"\n```",
				)
			}
		}
	}

	raven.SetUserContext(&raven.User{
		ID:       msg.ID,
		Username: msg.Author.Username + "#" + msg.Author.Discriminator,
	})

	raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{
		"ChannelID":       msg.ChannelID,
		"Content":         msg.Content,
		"Timestamp":       string(msg.Timestamp),
		"TTS":             strconv.FormatBool(msg.Tts),
		"MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
		"IsBot":           strconv.FormatBool(msg.Author.Bot),
	})
}
