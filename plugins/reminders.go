package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
    "time"
    "strings"
    "regexp"
    "strconv"
    rethink "gopkg.in/gorethink/gorethink.v3"
    "github.com/sn0w/Karen/utils"
    "github.com/sn0w/Karen/helpers"
)

type Reminders struct{}

type DB_Reminders struct {
    Id        string `gorethink:"id,omitempty"`
    UserID    string `gorethink:"userid"`
    Reminders []DB_Reminder `gorethink:"reminders"`
}

type DB_Reminder struct {
    Message   string `gorethink:"message"`
    ChannelID string `gorethink:"channelID"`
    GuildID   string `gorethink:"guildID"`
    Timestamp int64 `gorethink:"timestamp"`
}

func (r Reminders) Commands() []string {
    return []string{
        "remind",
        "rm",
        "reminders",
        "rms",
    }
}

func (r Reminders) Init(session *discordgo.Session) {
    go func() {
        for {
            var reminderBucket []DB_Reminders
            cursor, err := rethink.Table("reminders").Run(utils.GetDB())
            helpers.Relax(err)

            err = cursor.All(&reminderBucket)
            helpers.Relax(err)

            for _, reminders := range reminderBucket {
                changes := false

                // Downward loop for in-loop element removal
                for idx := len(reminders.Reminders) - 1; idx >= 0; idx-- {
                    reminder := reminders.Reminders[idx]

                    if reminder.Timestamp <= time.Now().Unix() {
                        session.ChannelMessageSend(
                            reminder.ChannelID,
                            ":alarm_clock: Ring! Ring! <@" + reminders.UserID + ">\n" +
                                "You wanted me to remind you to `" + reminder.Message + "` :slight_smile:",
                        )

                        reminders.Reminders = helpers.SliceRemoveOrderedElement(reminders.Reminders, idx).([]DB_Reminder)
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

    fmt.Println("=> Started reminder loop (10s)")
}

func (r Reminders) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch command {
    case "rm", "remind":
        parts := strings.Split(content, " ")

        if len(parts) < 4 {
            session.ChannelMessageSend(msg.ChannelID, ":x: Please check if the format is correct")
            return
        }

        unitRegex := regexp.MustCompile(`^(s|seconds|m|minutes|h|hours|d|days)$`)

        unit := parts[len(parts) - 1]

        schedule, err := strconv.ParseInt(
            regexp.MustCompile(`\D`).ReplaceAllString(parts[len(parts) - 2], ""),
            10,
            64,
        )

        if err != nil {
            session.ChannelMessageSend(msg.ChannelID, ":x: Please check if the time-format is correct")
        }

        message := strings.Join(parts[0:len(parts) - 3], " ")

        if !unitRegex.MatchString(unit) {
            session.ChannelMessageSend(msg.ChannelID, ":x: Please check if the time-format is correct")
            return
        }

        ts := time.Now().Unix()

        switch unit {
        case "s", "seconds":
            ts += schedule
            break

        case "m", "minutes":
            ts += schedule * 60
            break

        case "h", "hours":
            ts += schedule * 60 * 60
            break

        case "d", "days":
            ts += schedule * 60 * 60 * 24
            break

        default:
            session.ChannelMessageSend(msg.ChannelID, ":x: Please check if the time-format is correct")
            return
        }

        channel, err := session.Channel(msg.ChannelID)
        if err != nil {
            panic(err)
        }

        reminders := getReminders(msg.Author.ID)
        reminders.Reminders = append(reminders.Reminders, DB_Reminder{
            Message: message,
            ChannelID: channel.ID,
            GuildID: channel.GuildID,
            Timestamp: ts,
        })
        setReminders(msg.Author.ID, reminders)

        session.ChannelMessageSend(msg.ChannelID, "Ok I'll remind you :ok_hand:")
        break

    case "rms", "reminders":
        reminders := getReminders(msg.Author.ID)

        m := "These are your pending reminders :slight_smile:\n"

        m += "```\n"
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

            m += fmt.Sprintf(
                "%s - %s | #%s in '%s'\n",
                ts.String(),
                reminder.Message,
                channel,
                guild,
            )
        }
        m += "\n```"

        session.ChannelMessageSend(msg.ChannelID, m)
        break
    }
}

func getReminders(uid string) DB_Reminders {
    var reminderBucket DB_Reminders
    listCursor, err := rethink.Table("reminders").Filter(
        rethink.Row.Field("userid").Eq(uid),
    ).Run(utils.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&reminderBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        _, e := rethink.Table("reminders").Insert(DB_Reminders{
            UserID: uid,
            Reminders: make([]DB_Reminder, 0),
        }).RunWrite(utils.GetDB())

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
    _, err := rethink.Table("reminders").Update(reminders).Run(utils.GetDB())
    helpers.Relax(err)
}
