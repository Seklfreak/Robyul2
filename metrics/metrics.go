package metrics

import (
    "expvar"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/bwmarrin/discordgo"
    rethink "github.com/gorethink/gorethink"
    "net/http"
    "runtime"
    "time"
)

var (
    // MessagesReceived counts all ever received messages
    MessagesReceived = expvar.NewInt("messages_received")

    // UserCount counts all logged-in users
    UserCount = expvar.NewInt("user_count")

    // ChannelCount counts all watching channels
    ChannelCount = expvar.NewInt("channel_count")

    // GuildCount counts all joined guilds
    GuildCount = expvar.NewInt("guild_count")

    // CommandsExecuted increases after each command execution
    CommandsExecuted = expvar.NewInt("commands_executed")

    // CleverbotRequests increases after each request to cleverbot.com
    CleverbotRequests = expvar.NewInt("cleverbot_requests")

    // CoroutineCount counts all running coroutines
    CoroutineCount = expvar.NewInt("coroutine_count")

    // Uptime stores the timestamp of the bot's boot
    Uptime = expvar.NewInt("uptime")

    // VliveChannelsCount counts all connected vlive channels
    VliveChannelsCount = expvar.NewInt("vlive_channels_count")

    // TwitterAccountsCount counts all connected twitter accounts
    TwitterAccountsCount = expvar.NewInt("twitter_accounts_count")

    // InstagramAccountsCount counts all connected instagram accounts
    InstagramAccountsCount = expvar.NewInt("instagram_accounts_count")

    // FacebookPagesCount counts all connected instagram accounts
    FacebookPagesCount = expvar.NewInt("facebook_pages_count")

    // WolframAlphaRequests increases after each request to wolframalpha.com
    WolframAlphaRequests = expvar.NewInt("wolframalpha_requests")

    // LastFmRequests increases after each request to last.fm
    LastFmRequests = expvar.NewInt("lastfm_requests")

    // DarkSkyRequests increases after each request to darksky.net
    DarkSkyRequests = expvar.NewInt("darksky_requests")

    // KeywordNotificationsSentCount increased after every keyword notification sent
    KeywordNotificationsSentCount = expvar.NewInt("keywordnotifications_sent_count")

    // GalleriesCount counts all galleries in the db
    GalleriesCount = expvar.NewInt("galleries_count")

    // GalleryPostsSent increased with every link reposted
    GalleryPostsSent = expvar.NewInt("gallery_posts_sent")

    // GalleriesCount counts all galleries in the db
    MirrorsCount = expvar.NewInt("mirrors_count")

    // GalleryPostsSent increased with every link reposted
    MirrorsPostsSent = expvar.NewInt("mirrors_posts_sent")

    // LevelImagesGeneratedCount increased with every level image generated
    LevelImagesGeneratedCount = expvar.NewInt("levels_images_generated_count")
)

// Init starts a http server on 127.0.0.1:1337
func Init() {
    logger.INFO.L("metrics", "Listening on TCP/1337")
    Uptime.Set(time.Now().Unix())
    go http.ListenAndServe(helpers.GetConfig().Path("metrics_ip").Data().(string)+":1337", nil)
}

// OnReady listens for said discord event
func OnReady(session *discordgo.Session, event *discordgo.Ready) {
    go CollectDiscordMetrics(session)
    go CollectRuntimeMetrics()
}

// OnMessageCreate listens for said discord event
func OnMessageCreate(session *discordgo.Session, event *discordgo.MessageCreate) {
    MessagesReceived.Add(1)
}

// CollectDiscordMetrics counts Guilds, Channels and Users
func CollectDiscordMetrics(session *discordgo.Session) {
    for {
        time.Sleep(15 * time.Second)

        users := make(map[string]string)
        channels := 0
        guilds := session.State.Guilds

        for _, guild := range guilds {
            channels += len(guild.Channels)

            lastAfterMemberId := ""
            for {
                members, err := session.GuildMembers(guild.ID, lastAfterMemberId, 1000)
                if len(members) <= 0 {
                    break
                }
                lastAfterMemberId = members[len(members)-1].User.ID
                helpers.Relax(err)
                for _, u := range members {
                    users[u.User.ID] = u.User.Username
                }
            }
        }

        UserCount.Set(int64(len(users)))
        ChannelCount.Set(int64(channels))
        GuildCount.Set(int64(len(guilds)))
    }
}

// CollectRuntimeMetrics counts all running coroutines
func CollectRuntimeMetrics() {
    for {
        time.Sleep(15 * time.Second)

        CoroutineCount.Set(int64(runtime.NumGoroutine()))

        var cnt int

        cursor, err := rethink.Table("vlive").Count().Run(helpers.GetDB())
        helpers.Relax(err)
        cursor.One(&cnt)
        cursor.Close()
        VliveChannelsCount.Set(int64(cnt))

        cursor, err = rethink.Table("instagram").Count().Run(helpers.GetDB())
        helpers.Relax(err)
        cursor.One(&cnt)
        cursor.Close()
        InstagramAccountsCount.Set(int64(cnt))

        cursor, err = rethink.Table("twitter").Count().Run(helpers.GetDB())
        helpers.Relax(err)
        cursor.One(&cnt)
        cursor.Close()
        TwitterAccountsCount.Set(int64(cnt))

        cursor, err = rethink.Table("facebook").Count().Run(helpers.GetDB())
        helpers.Relax(err)
        cursor.One(&cnt)
        cursor.Close()
        FacebookPagesCount.Set(int64(cnt))

        cursor, err = rethink.Table("galleries").Count().Run(helpers.GetDB())
        helpers.Relax(err)
        cursor.One(&cnt)
        cursor.Close()
        GalleriesCount.Set(int64(cnt))

        cursor, err = rethink.Table("mirrors").Count().Run(helpers.GetDB())
        helpers.Relax(err)
        cursor.One(&cnt)
        cursor.Close()
        MirrorsCount.Set(int64(cnt))
    }
}
