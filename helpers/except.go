// Except.go: Contains functions to make handling panics less PITA

package helpers

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"

	"math/rand"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"github.com/getsentry/raven-go"
	"github.com/olivere/elastic"
)

// RecoverDiscord recover()s and sends a message to discord
func RecoverDiscord(msg *discordgo.Message) {
	err := recover()
	if err != nil {
		if strings.Contains(fmt.Sprintf("%+#v", err), "handled discord error") {
			return
		}

		fmt.Printf("RecoverDiscord: %s\n", spew.Sdump(err))

		SendError(msg, err)
	}
}

// Recover recover()s and prints the error to console
func Recover() {
	err := recover()
	if err != nil {
		if strings.Contains(fmt.Sprintf("%+#v", err), "handled discord error") {
			return
		}

		fmt.Printf("Recover: %s\n", spew.Sdump(err))

		//raven.SetUserContext(&raven.User{})
		if errE, ok := err.(*elastic.Error); ok {
			raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{
				"Type":     errE.Details.Type,
				"Reason":   errE.Details.Reason,
				"Index":    errE.Details.Index,
				"CausedBy": spew.Sdump(errE.Details.CausedBy),
			})
		} else {
			raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{})
		}
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
			spew.Dump(err)

			buf := make([]byte, 1<<16)
			stackSize := runtime.Stack(buf, false)

			fmt.Println(string(buf[0:stackSize]))

			if errD, ok := err.(*discordgo.RESTError); ok && errD != nil && errD.Message != nil {
				fmt.Println(strconv.Itoa(errD.Message.Code)+":", errD.Message.Message)
			}

			panic(err)
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
					_, err = SendMessage(channelID, GetText("bot.errors.no-embed"))
					RelaxMessage(err, channelID, commandMessageID)
				}
				panic("handled discord error")
				return
			}
		}
		Relax(err)
	}
}

// RelaxEmbed does nothing if $err is nil or if there are no permissions to send a message, else sends it to Relax()
func RelaxMessage(err error, channelID string, commandMessageID string) {
	if err != nil {
		if errD, ok := err.(*discordgo.RESTError); ok && errD != nil {
			if errD.Message.Code == 50013 {
				if channelID != "" && commandMessageID != "" {
					reactions := []string{
						":blobstop:317034621953114112",
						"a:ablobweary:394026914479865856",
						":googlespeaknoevil:317036753074651139",
						":notlikeblob:349342777978519562",
						"a:ablobcry:393869333740126219",
						"a:ablobfrown:394026913292615701",
						"a:ablobunamused:393869335573037057",
					}
					cache.GetSession().MessageReactionAdd(channelID, commandMessageID, reactions[rand.Intn(len(reactions))])
				}
				panic("handled discord error")
				return
			} else {
				Relax(err)
			}
		} else {
			Relax(err)
		}
	}
}

func RelaxLog(err error) {
	if err != nil {
		fmt.Printf("Error: %s\n", spew.Sdump(err))

		raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{})
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

		SendMessage(
			msg.ChannelID,
			"Error <a:ablobfrown:394026913292615701>\n```\n"+spew.Sdump(err)+fmt.Sprintf("%s\n", string(buf[0:stackSize]))+"\n```",
		)
	} else {
		if errR, ok := err.(*discordgo.RESTError); ok && errR != nil && errR.Message != nil {
			if msg != nil {
				SendMessage(
					msg.ChannelID,
					"Error <a:ablobfrown:394026913292615701>\n```\n"+fmt.Sprintf("%+#v", errR.Message.Message)+"\n```",
				)
			}
		} else {
			if msg != nil {
				SendMessage(
					msg.ChannelID,
					"Error <a:ablobfrown:394026913292615701>\n```\n"+fmt.Sprintf("%+#v", err)+"\n```",
				)
			}
		}
	}

	raven.SetUserContext(&raven.User{
		ID:       msg.ID,
		Username: msg.Author.Username + "#" + msg.Author.Discriminator,
	})

	raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{
		"ChannelID":       msg.ChannelID,
		"Content":         msg.Content,
		"Timestamp":       string(msg.Timestamp),
		"TTS":             strconv.FormatBool(msg.Tts),
		"MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
		"IsBot":           strconv.FormatBool(msg.Author.Bot),
	})
}
