package plugins

import (
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/olebedev/when"
    "github.com/olebedev/when/rules/en"
    "github.com/olebedev/when/rules/common"
    "github.com/bwmarrin/discordgo"
    rethink "github.com/gorethink/gorethink"
    "strings"
    "time"
)

type Reminders struct {
    parser *when.Parser
}

type DB_Reminders struct {
    Id        string        `gorethink:"id,omitempty"`
    UserID    string        `gorethink:"userid"`
    Reminders []DB_Reminder `gorethink:"reminders"`
}

type DB_Reminder struct {
    Message   string `gorethink:"message"`
    ChannelID string `gorethink:"channelID"`
    GuildID   string `gorethink:"guildID"`
    Timestamp int64  `gorethink:"timestamp"`
}

func (r *Reminders) Commands() []string {
    return []string{
        "remind",
        "remindme",
        "rm",
        "reminders",
        "rms",
    }
}

func (r *Reminders) Init(session *discordgo.Session) {
    r.parser = when.New(nil)
    r.parser.Add(en.All...)
    r.parser.Add(common.All...)

    go func() {
        defer helpers.Recover()

        for {
            reminderBucket := make([]DB_Reminders, 0)
            cursor, err := rethink.Table("reminders").Run(helpers.GetDB())
            helpers.Relax(err)

            err = cursor.All(&reminderBucket)
            helpers.Relax(err)

            for _, reminders := range reminderBucket {
                changes := false

                // Downward loop for in-loop element removal
                for idx := len(reminders.Reminders) - 1; idx >= 0; idx-- {
                    reminder := reminders.Reminders[idx]

                    if reminder.Timestamp <= time.Now().Unix() {
                        dmChannel, err := session.UserChannelCreate(reminders.UserID)
                        helpers.Relax(err)

                        session.ChannelMessageSend(
                            dmChannel.ID,
                            ":alarm_clock: You wanted me to remind you about this:\n"+"```"+reminder.Message+"```",
                        )

                        reminders.Reminders = append(reminders.Reminders[:idx], reminders.Reminders[idx+1:]...)
                        changes = true
                    }
                }

                if changes {
                    setReminders(reminders.UserID, reminders)
                }
            }

            time.Sleep(10 * time.Second)
        }
    }()

    logger.PLUGIN.L("reminders", "Started reminder loop (10s)")
}

func (r *Reminders) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch command {
    case "rm", "remind", "remindme":
        channel, err := cache.Channel(msg.ChannelID)
        helpers.Relax(err)

        parts := strings.Fields(content)

        if len(parts) < 3 {
            session.ChannelMessageSend(msg.ChannelID, ":x: Please check if the format is correct")
            return
        }

        r, err := r.parser.Parse(content, time.Now())
        helpers.Relax(err)
        if r == nil {
            session.ChannelMessageSend(msg.ChannelID, ":x: Please check if the format is correct")
            return
        }

        reminders := getReminders(msg.Author.ID)
        reminders.Reminders = append(reminders.Reminders, DB_Reminder{
            Message:   strings.Replace(content, r.Text, "", 1),
            ChannelID: channel.ID,
            GuildID:   channel.GuildID,
            Timestamp: r.Time.Unix(),
        })
        setReminders(msg.Author.ID, reminders)

        session.ChannelMessageSend(msg.ChannelID, "Ok I'll remind you :ok_hand:")
        break

    case "rms", "reminders":
        reminders := getReminders(msg.Author.ID)
        embedFields := []*discordgo.MessageEmbedField{}

        for _, reminder := range reminders.Reminders {
            ts := time.Unix(reminder.Timestamp, 0)
            channel := "?"
            guild := "?"

            chanRef, err := session.Channel(reminder.ChannelID)
            if err == nil {
                channel = chanRef.Name
            }

            guildRef, err := session.Guild(reminder.GuildID)
            if err == nil {
                guild = guildRef.Name
            }

            embedFields = append(embedFields, &discordgo.MessageEmbedField{
                Inline: false,
                Name:   reminder.Message,
                Value:  "At " + ts.String() + " in #" + channel + " of " + guild,
            })
        }

        if len(embedFields) == 0 {
            session.ChannelMessageSend(msg.ChannelID, helpers.GetText("remiders.empty"))
            return
        }

        session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
            Title:  "Pending reminders",
            Fields: embedFields,
            Color:  0x0FADED,
        })
        break
    }
}

func getReminders(uid string) DB_Reminders {
    var reminderBucket DB_Reminders
    listCursor, err := rethink.Table("reminders").Filter(
        rethink.Row.Field("userid").Eq(uid),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&reminderBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        _, e := rethink.Table("reminders").Insert(DB_Reminders{
            UserID:    uid,
            Reminders: make([]DB_Reminder, 0),
        }).RunWrite(helpers.GetDB())

        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return getReminders(uid)
        }
    } else if err != nil {
        panic(err)
    }

    return reminderBucket
}

func setReminders(uid string, reminders DB_Reminders) {
    _, err := rethink.Table("reminders").Update(reminders).Run(helpers.GetDB())
    helpers.Relax(err)
}
