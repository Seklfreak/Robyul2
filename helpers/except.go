// Except.go: Contains functions to make handling panics less PITA

package helpers

import (
    "fmt"
    "git.lukas.moe/sn0w/Karen/cache"
    "github.com/getsentry/raven-go"
    "github.com/sn0w/discordgo"
    "reflect"
    "runtime"
    "strconv"
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
        panic(err)
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
        buf := make([]byte, 1 << 16)
        stackSize := runtime.Stack(buf, false)

        cache.GetSession().ChannelMessageSend(
            msg.ChannelID,
            "Error :frowning:\n0xFADED#3237 has been notified.\n```\n" +
                fmt.Sprintf("%#v\n", err) +
                fmt.Sprintf("%s\n", string(buf[0:stackSize])) +
                "\n```\nhttp://i.imgur.com/FcV2n4X.jpg",
        )
    } else {
        cache.GetSession().ChannelMessageSend(
            msg.ChannelID,
            "Error :frowning:\n0xFADED#3237 has been notified.\n```\n" +
                fmt.Sprintf("%#v", err) +
                "\n```\nhttp://i.imgur.com/FcV2n4X.jpg",
        )
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
