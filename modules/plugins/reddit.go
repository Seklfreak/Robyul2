package plugins

import (
	"errors"
	"strings"

	"fmt"

	"time"

	"strconv"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/version"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
	"github.com/jzelinskie/geddit"
)

type redditAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next redditAction)

type Reddit struct{}

var (
	redditSession   *geddit.OAuthSession
	RedditUserAgent string = "geddit:Robyul:" + version.BOT_VERSION + " by /u/Seklfreak"
)

const (
	RedditBaseUrl = "https://www.reddit.com"
	RedditColor   = "ff4500"
)

func (r *Reddit) Commands() []string {
	return []string{
		"reddit",
	}
}

func (r *Reddit) Init(session *discordgo.Session) {
	var err error
	redditSession, err = geddit.NewOAuthSession(
		helpers.GetConfig().Path("reddit.id").Data().(string),
		helpers.GetConfig().Path("reddit.secret").Data().(string),
		RedditUserAgent,
		"https://robyul.chat",
	)
	helpers.Relax(err)
	err = redditSession.LoginAuth(
		helpers.GetConfig().Path("reddit.username").Data().(string),
		helpers.GetConfig().Path("reddit.password").Data().(string),
	)
	helpers.Relax(err)
	go r.checkSubredditLoop()
	r.logger().Info("Started checkSubredditLoop loop (0s)")
}

func (r *Reddit) checkSubredditLoop() {
	defer helpers.Recover()
	defer func() {
		go func() {
			r.logger().Error("The checkSubredditLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			r.checkSubredditLoop()
		}()
	}()

	var entries []models.RedditSubredditEntry
	var bundledEntries map[string][]models.RedditSubredditEntry

	for {
		cursor, err := rethink.Table(models.RedditSubredditsTable).Run(helpers.GetDB())
		helpers.Relax(err)

		err = cursor.All(&entries)
		helpers.Relax(err)

		bundledEntries = make(map[string][]models.RedditSubredditEntry, 0)

		for _, entry := range entries {
			channel, err := helpers.GetChannel(entry.ChannelID)
			if err != nil || channel == nil || channel.ID == "" {
				r.logger().Warn(fmt.Sprintf("skipped subreddit r/%s for Channel #%s on Guild #%s: channel not found!",
					entry.SubredditName, entry.ChannelID, entry.GuildID))
				continue
			}

			if _, ok := bundledEntries[entry.SubredditName]; ok {
				bundledEntries[entry.SubredditName] = append(bundledEntries[entry.SubredditName], entry)
			} else {
				bundledEntries[entry.SubredditName] = []models.RedditSubredditEntry{entry}
			}
		}

		for subredditName, entries := range bundledEntries {
		BundleStart:
			r.logger().Info(fmt.Sprintf("checking subreddit r/%s for %d channels", subredditName, len(entries)))
			newSubmissions, err := redditSession.SubredditSubmissions(subredditName, geddit.NewSubmissions, geddit.ListingOptions{
				Limit: 30,
			})
			if err != nil {
				if strings.Contains(err.Error(), "oauth2: token expired and refresh token is not set") {
					// login when token expired
					err = redditSession.LoginAuth(
						helpers.GetConfig().Path("reddit.username").Data().(string),
						helpers.GetConfig().Path("reddit.password").Data().(string),
					)
					helpers.Relax(err)
					r.logger().Error("logged in again after token expired")
					goto BundleStart
				}
				r.logger().Error(fmt.Sprintf("updating subreddit r/%s failed: %s", subredditName, err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}
			for _, entry := range entries {
				hasToBeBefore := time.Now().Add(-(time.Duration(entry.PostDelay) * time.Minute))
				hasToBeAfter := entry.LastChecked

				for _, submission := range newSubmissions {
					submissionTime := time.Unix(int64(submission.DateCreated), 0)
					if !submissionTime.Before(hasToBeBefore) || !submissionTime.After(hasToBeAfter) {
						continue
					}

					postSubmission := submission
					postChannelID := entry.ChannelID
					go func() {
						defer helpers.Recover()

						r.logger().Info(fmt.Sprintf("posting submission: #%s (%s) on r/%s (%s) to #%s",
							postSubmission.ID, submissionTime.Format(time.ANSIC), subredditName, RedditBaseUrl+postSubmission.Permalink, entry.ChannelID))

						err = r.postSubmission(postChannelID, postSubmission)
						if err != nil {
							if errD, ok := err.(*discordgo.RESTError); !ok || errD.Message.Code != discordgo.ErrCodeMissingPermissions {
								helpers.Relax(err)
							}
						}
					}()
				}
				entry.LastChecked = hasToBeBefore
				err = r.setSubredditEntry(entry)
				helpers.Relax(err)
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func (r *Reddit) postSubmission(channelID string, submission *geddit.Submission) (err error) {
	data := &discordgo.MessageSend{}

	data.Content = "<" + RedditBaseUrl + submission.Permalink + ">"

	data.Embed = &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{
			Text: helpers.GetText("plugins.reddit.embed-footer") + " | r/" + submission.Subreddit + " | reddit #" + submission.ID},
		URL:    RedditBaseUrl + submission.Permalink,
		Author: &discordgo.MessageEmbedAuthor{Name: submission.Author, URL: RedditBaseUrl + "/u/" + submission.Author},
		Color:  helpers.GetDiscordColorFromHex(RedditColor),
	}

	data.Embed.Title = submission.Title
	if len(data.Embed.Title) > 128 {
		data.Embed.Title = submission.Title[0:127] + "…"
	}
	if submission.Selftext != "" {
		data.Embed.Description = submission.Selftext
		if len(data.Embed.Description) > 500 {
			data.Embed.Description = data.Embed.Description[0:499] + "…"
		}
	}
	if submission.ThumbnailURL != "" && strings.HasPrefix(submission.ThumbnailURL, "http") {
		data.Embed.Image = &discordgo.MessageEmbedImage{URL: submission.ThumbnailURL}
	}

	_, err = cache.GetSession().ChannelMessageSendComplex(channelID, data)
	return err
}

func (r *Reddit) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := r.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (r *Reddit) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	if len(args) < 1 {
		*out = r.newMsg("bot.arguments.too-few")
		return r.actionFinish
	}

	switch args[0] {
	case "add":
		return r.actionAdd
	case "delete", "remove":
		return r.actionRemove
	case "list":
		return r.actionList
	default:
		return r.actionInfo
	}
}

func (r *Reddit) actionAdd(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	var err error
	if !helpers.IsMod(in) {
		*out = r.newMsg(helpers.GetText("mod.no_permission"))
		return r.actionFinish
	}

	if len(args) < 3 {
		*out = r.newMsg("bot.arguments.too-few")
		return r.actionFinish
	}

	var postDelay int
	if len(args) > 3 {
		postDelay, err = strconv.Atoi(args[3])
		if err != nil {
			*out = r.newMsg("bot.arguments.invalid")
			return r.actionFinish
		}
	}

	targetChannel, err := helpers.GetChannelFromMention(in, args[2])
	if err != nil {
		*out = r.newMsg("bot.arguments.invalid")
		return r.actionFinish
	}

	subredditName := strings.TrimLeft(args[1], "/")
	subredditName = strings.Replace(subredditName, "r/", "", -1)

	subredditData, err := redditSession.AboutSubreddit(subredditName)
	helpers.Relax(err)

	if subredditData.ID == "" {
		*out = r.newMsg("plugins.reddit.subreddit-not-found")
		return r.actionFinish
	}

	_, err = r.addSubredditEntry(subredditData.Name, targetChannel.GuildID, targetChannel.ID, in.Author.ID, postDelay)
	helpers.Relax(err)

	// TODO: Post preview post

	*out = r.newMsg(helpers.GetTextF("plugins.reddit.add-subreddit-success", subredditData.Name, targetChannel.ID))
	return r.actionFinish
}

func (r *Reddit) actionList(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	subredditEntries, _ := r.getAllSubredditEntriesBy("guild_id", channel.GuildID)

	if len(subredditEntries) <= 0 {
		*out = r.newMsg("plugins.reddit.list-none")
		return r.actionFinish
	}

	subredditListText := ""
	for _, subredditEntry := range subredditEntries {
		subredditListText += fmt.Sprintf("`%s`: Subreddit `r/%s` posting to <#%s> (Delay: %d minutes)\n",
			subredditEntry.ID, subredditEntry.SubredditName, subredditEntry.ChannelID, subredditEntry.PostDelay)
	}
	subredditListText += fmt.Sprintf("Found **%d** Subreddits in total.", len(subredditEntries))

	// TODO: pagify

	*out = r.newMsg(subredditListText)
	return r.actionFinish
}

func (r *Reddit) actionRemove(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	if !helpers.IsMod(in) {
		*out = r.newMsg(helpers.GetText("mod.no_permission"))
		return r.actionFinish
	}

	if len(args) < 2 {
		*out = r.newMsg("bot.arguments.too-few")
		return r.actionFinish
	}

	subredditEntry, err := r.getSubredditEntryBy("id", args[1])
	if err != nil || subredditEntry.ID == "" {
		*out = r.newMsg("plugins.reddit.remove-subreddit-error-not-found")
		return r.actionFinish
	}

	err = r.removeSubredditEntry(subredditEntry)
	helpers.Relax(err)

	*out = r.newMsg(helpers.GetTextF("plugins.reddit.remove-subreddit-success", subredditEntry.SubredditName))
	return r.actionFinish
}

func (r *Reddit) actionInfo(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	subredditName := strings.TrimLeft(args[0], "/")
	subredditName = strings.Replace(subredditName, "r/", "", -1)

	*out = r.getSubredditInfo(subredditName)
	return r.actionFinish
}

func (r *Reddit) getSubredditInfo(subreddit string) (data *discordgo.MessageSend) {
	subredditData, err := redditSession.AboutSubreddit(subreddit)
	if err != nil {
		return r.newMsg(err.Error())
	}

	if subredditData.ID == "" {
		return r.newMsg("plugins.reddit.subreddit-not-found")
	}

	data = &discordgo.MessageSend{}

	isNSFWText := "No"
	if subredditData.IsNSFW {
		isNSFWText = "Yes"
	}
	titleText := "r/" + subredditData.Name
	if subredditData.Title != "" {
		titleText += " ~ " + subredditData.Title
	}

	creationTime := time.Unix(int64(subredditData.DateCreated), 0)

	data.Embed = &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{
			Text: helpers.GetText("plugins.reddit.embed-footer") + " | reddit #" + subredditData.ID},
		Title:       titleText,
		Description: subredditData.PublicDesc,
		URL:         RedditBaseUrl + subredditData.URL,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Subscribers", Value: humanize.Comma(int64(subredditData.NumSubs)), Inline: true},
			{Name: "Creation", Value: fmt.Sprintf("%s. That's %s.",
				creationTime.Format(time.ANSIC), helpers.SinceInDaysText(creationTime)), Inline: true},
			{Name: "NSFW?", Value: isNSFWText, Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(RedditColor),
	}

	if subredditData.HeaderImg != "" {
		data.Embed.Image = &discordgo.MessageEmbedImage{URL: subredditData.HeaderImg}
	}

	return
}

func (r *Reddit) addSubredditEntry(subreddit string, guildID string, channelID string, userID string, postDelay int) (subredditEntry models.RedditSubredditEntry, err error) {
	insert := rethink.Table(models.RedditSubredditsTable).Insert(models.RedditSubredditEntry{
		SubredditName: subreddit,
		GuildID:       guildID,
		ChannelID:     channelID,
		AddedByUserID: userID,
		AddedAt:       time.Now(),
		LastChecked:   time.Now().Add(-(time.Duration(postDelay) * time.Minute)),
		PostDelay:     postDelay,
	})
	res, err := insert.RunWrite(helpers.GetDB())
	if err != nil {
		return subredditEntry, err
	} else {
		return r.getSubredditEntryBy("id", res.GeneratedKeys[0])
	}
}

func (r *Reddit) setSubredditEntry(subredditEntry models.RedditSubredditEntry) (err error) {
	if subredditEntry.ID != "" {
		_, err := rethink.Table(models.RedditSubredditsTable).Update(subredditEntry).Run(helpers.GetDB())
		return err
	}
	return errors.New("empty subreddit entry submitted")
}

func (r *Reddit) getSubredditEntryBy(key string, value string) (subredditEntry models.RedditSubredditEntry, err error) {
	listCursor, err := rethink.Table(models.RedditSubredditsTable).Filter(
		rethink.Row.Field(key).Eq(value),
	).Run(helpers.GetDB())
	if err != nil {
		return subredditEntry, err
	}
	defer listCursor.Close()
	err = listCursor.One(&subredditEntry)

	if err == rethink.ErrEmptyResult {
		return subredditEntry, errors.New("no subreddit entry")
	} else if err != nil {
		return subredditEntry, err
	}

	return subredditEntry, nil
}

func (r *Reddit) getAllSubredditEntriesBy(key string, value string) (subredditEntry []models.RedditSubredditEntry, err error) {
	listCursor, err := rethink.Table(models.RedditSubredditsTable).Filter(
		rethink.Row.Field(key).Eq(value),
	).Run(helpers.GetDB())
	if err != nil {
		return subredditEntry, err
	}
	defer listCursor.Close()
	err = listCursor.All(&subredditEntry)

	if err == rethink.ErrEmptyResult {
		return subredditEntry, errors.New("no subreddit entries")
	} else if err != nil {
		return subredditEntry, err
	}

	return subredditEntry, nil
}

func (r *Reddit) removeSubredditEntry(subredditEntry models.RedditSubredditEntry) error {
	if subredditEntry.ID != "" {
		_, err := rethink.Table(models.RedditSubredditsTable).Get(subredditEntry.ID).Delete().RunWrite(helpers.GetDB())
		return err
	}
	return errors.New("empty subreddit entry submitted")
}

func (r *Reddit) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	_, err := cache.GetSession().ChannelMessageSendComplex(in.ChannelID, *out)
	helpers.RelaxMessage(err, in.ChannelID, in.ID)

	return nil
}

func (r *Reddit) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (r *Reddit) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (r *Reddit) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "reddit")
}
