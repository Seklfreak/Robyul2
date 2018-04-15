package plugins

import (
	"strings"

	"fmt"

	"time"

	"strconv"

	"html"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/version"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
	"github.com/jzelinskie/geddit"
	"github.com/sirupsen/logrus"
)

type redditAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next redditAction)

type Reddit struct{}

var (
	redditSession   *geddit.OAuthSession
	RedditUserAgent = "geddit:Robyul:" + version.BOT_VERSION + " by /u/Seklfreak"
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
	var newPost bool

	for {
		err := helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.RedditSubredditsTable).Find(nil)).All(&entries)
		helpers.Relax(err)

		bundledEntries = make(map[string][]models.RedditSubredditEntry, 0)

		for _, entry := range entries {
			channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
			if err != nil || channel == nil || channel.ID == "" {
				//r.logger().Warn(fmt.Sprintf("skipped subreddit r/%s for Channel #%s on Guild #%s: channel not found!",
				//	entry.SubredditName, entry.ChannelID, entry.GuildID))
				continue
			}

			if _, ok := bundledEntries[entry.SubredditName]; ok {
				bundledEntries[entry.SubredditName] = append(bundledEntries[entry.SubredditName], entry)
			} else {
				bundledEntries[entry.SubredditName] = []models.RedditSubredditEntry{entry}
			}
		}

		r.logger().Infof("checking %d subreddits for %d feeds", len(bundledEntries), len(entries))

		for subredditName, entries := range bundledEntries {
		BundleStart:
			// r.logger().Info(fmt.Sprintf("checking subreddit r/%s for %d channels", subredditName, len(entries)))
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
					r.logger().Warn("logged in again after token expired")
					goto BundleStart
				}
				r.logger().Warnf("updating subreddit r/%s failed: %s", subredditName, err.Error())
				time.Sleep(2 * time.Second)
				continue
			}
			for _, entry := range entries {
				newPost = false
				hasToBeBefore := time.Now().Add(-(time.Duration(entry.PostDelay) * time.Minute))
				hasToBeAfter := entry.LastChecked

				for _, submission := range newSubmissions {
					submissionTime := time.Unix(int64(submission.DateCreated), 0)
					if !submissionTime.Before(hasToBeBefore) || !submissionTime.After(hasToBeAfter) {
						continue
					}
					newPost = true

					postSubmission := submission
					postChannelID := entry.ChannelID
					go func() {
						defer helpers.Recover()

						r.logger().Info(fmt.Sprintf("posting submission: #%s (%s) on r/%s (%s) to #%s",
							postSubmission.ID, submissionTime.Format(time.ANSIC), subredditName,
							RedditBaseUrl+"/r/"+subredditName+"/comments/"+postSubmission.ID+"/", entry.ChannelID))

						err = r.postSubmission(postChannelID, postSubmission, entry.PostDirectLinks)
						if err != nil {
							if errD, ok := err.(*discordgo.RESTError); ok && errD.Message != nil {
								if errD.Message.Code != discordgo.ErrCodeMissingPermissions &&
									errD.Message.Code != discordgo.ErrCodeUnknownChannel {
									helpers.Relax(err)
								}
							} else {
								helpers.Relax(err)
							}
						}
					}()
				}
				if newPost {
					entry.LastChecked = hasToBeBefore
					err = helpers.MDbUpdateWithoutLogging(models.RedditSubredditsTable, entry.ID, entry)
					helpers.Relax(err)
				}
			}
			time.Sleep(2 * time.Second)
		}

		if len(entries) <= 10 {
			time.Sleep(time.Second * 60)
		}
	}
}

func (r *Reddit) postSubmission(channelID string, submission *geddit.Submission, postDirectLinks bool) (err error) {
	data := &discordgo.MessageSend{}

	data.Content = "<" + RedditBaseUrl + submission.Permalink + ">"

	var content string
	data.Embed = &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.reddit.embed-footer") + " | /r/" + submission.Subreddit + " | reddit #" + submission.ID,
			IconURL: helpers.GetText("plugins.reddit.embed-footer-imageurl"),
		},
		URL:    RedditBaseUrl + submission.Permalink,
		Author: &discordgo.MessageEmbedAuthor{Name: "/u/" + submission.Author, URL: RedditBaseUrl + "/u/" + submission.Author},
		Color:  helpers.GetDiscordColorFromHex(RedditColor),
	}

	var textModeTitle, textModeSelftext string

	data.Embed.Title = submission.Title
	textModeTitle = "**" + submission.Title
	if submission.LinkFlairText != "" {
		textModeTitle = "`[" + submission.LinkFlairText + "]` **" + data.Embed.Title
		data.Embed.Title = "`" + submission.LinkFlairText + "` " + data.Embed.Title

	}
	data.Embed.Title = html.UnescapeString(data.Embed.Title)
	if len(data.Embed.Title) > 128 {
		data.Embed.Title = data.Embed.Title[0:127] + "…"
	}
	textModeTitle = html.UnescapeString(textModeTitle)
	if len(textModeTitle) > 128 {
		textModeTitle = textModeTitle[0:127] + "…"
	}
	textModeTitle += "**"
	if submission.Selftext != "" {
		data.Embed.Description = html.UnescapeString(submission.Selftext)
		if len(data.Embed.Description) > 500 {
			data.Embed.Description = data.Embed.Description[0:499] + "…"
		}
		textModeSelftext = data.Embed.Description
	}
	if strings.HasSuffix(strings.ToLower(submission.URL), ".jpg") ||
		strings.HasSuffix(strings.ToLower(submission.URL), ".jpeg") ||
		strings.HasSuffix(strings.ToLower(submission.URL), ".gif") ||
		strings.HasSuffix(strings.ToLower(submission.URL), ".png") {
		data.Embed.Image = &discordgo.MessageEmbedImage{URL: submission.URL}
	} else if submission.ThumbnailURL != "" && strings.HasPrefix(submission.ThumbnailURL, "http") {
		data.Embed.Image = &discordgo.MessageEmbedImage{URL: submission.ThumbnailURL}
	}

	if postDirectLinks {
		content += textModeTitle + " _" + helpers.GetText("plugins.reddit.embed-footer") + "_\n"
		content += "<" + RedditBaseUrl + submission.Permalink + "> by `/u/" + submission.Author + "`\n"
		if textModeSelftext != "" {
			content += textModeSelftext + "\n"
		}
		if submission.URL != RedditBaseUrl+submission.Permalink {
			content += submission.URL
		}
		data.Content = content
		data.Embed = nil
	}

	_, err = helpers.SendComplex(channelID, data)
	return err
}

func (r *Reddit) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermReddit) {
		return
	}

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
	case "toggle-direct-link", "toggle-direct-links":
		return r.actionToggleDirectLinks
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
			postDelay = 0
		}
	}

	targetChannel, err := helpers.GetChannelFromMention(in, args[2])
	if err != nil {
		*out = r.newMsg("bot.arguments.invalid")
		return r.actionFinish
	}

	subredditName := strings.TrimLeft(args[1], "/")
	subredditName = strings.Replace(subredditName, "r/", "", -1)

	var specialText string
	if postDelay > 0 {
		specialText += fmt.Sprintf(" with a %d minutes delay", postDelay)
	}

	var linkMode bool
	if strings.HasSuffix(in.Content, " direct link mode") ||
		strings.HasSuffix(in.Content, " link mode") ||
		strings.HasSuffix(in.Content, " links") {
		linkMode = true
		specialText += " using direct links"
	}

	subredditData, err := redditSession.AboutSubreddit(subredditName)
	helpers.Relax(err)

	if subredditData.ID == "" {
		*out = r.newMsg("plugins.reddit.subreddit-not-found")
		return r.actionFinish
	}

	newID, err := helpers.MDbInsert(models.RedditSubredditsTable,
		models.RedditSubredditEntry{
			SubredditName:   subredditData.Name,
			LastChecked:     time.Now().Add(-(time.Duration(postDelay) * time.Minute)),
			GuildID:         targetChannel.GuildID,
			ChannelID:       targetChannel.ID,
			AddedByUserID:   in.Author.ID,
			AddedAt:         time.Now(),
			PostDelay:       postDelay,
			PostDirectLinks: linkMode,
		})
	helpers.Relax(err)

	_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, helpers.MdbIdToHuman(newID),
		models.EventlogTargetTypeRobyulRedditFeed, in.Author.ID,
		models.EventlogTypeRobyulRedditFeedAdd, "",
		nil,
		[]models.ElasticEventlogOption{
			{
				Key:   "reddit_channelid",
				Value: targetChannel.ID,
				Type:  models.EventlogTargetTypeChannel,
			},
			{
				Key:   "reddit_postdirectlinks",
				Value: helpers.StoreBoolAsString(linkMode),
			},
			{
				Key:   "reddit_postdelay",
				Value: strconv.Itoa(postDelay),
			},
			{
				Key:   "reddit_subredditname",
				Value: subredditData.Name,
			},
		}, false)
	helpers.RelaxLog(err)

	// TODO: Post preview post

	*out = r.newMsg("plugins.reddit.add-subreddit-success", subredditData.Name, targetChannel.ID, specialText)
	return r.actionFinish
}

func (r *Reddit) actionList(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	var subredditEntries []models.RedditSubredditEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.RedditSubredditsTable).Find(bson.M{"guildid": channel.GuildID})).All(&subredditEntries)
	helpers.Relax(err)

	if subredditEntries == nil || len(subredditEntries) <= 0 {
		*out = r.newMsg("plugins.reddit.list-none")
		return r.actionFinish
	}

	subredditListText := ""
	for _, subredditEntry := range subredditEntries {
		var directLinkModeText string
		if subredditEntry.PostDirectLinks {
			directLinkModeText = ", direct link mode"
		}

		subredditListText += fmt.Sprintf("`%s`: Subreddit `r/%s` posting to <#%s> (Delay: %d minutes%s)\n",
			helpers.MdbIdToHuman(subredditEntry.ID), subredditEntry.SubredditName, subredditEntry.ChannelID,
			subredditEntry.PostDelay, directLinkModeText)
	}
	subredditListText += fmt.Sprintf("Found **%d** Subreddits in total.", len(subredditEntries))

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

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	var subredditEntry models.RedditSubredditEntry
	err = helpers.MdbOne(
		helpers.MdbCollection(models.RedditSubredditsTable).Find(bson.M{"guildid": channel.GuildID, "_id": helpers.HumanToMdbId(args[1])}),
		&subredditEntry,
	)
	if helpers.IsMdbNotFound(err) {
		*out = r.newMsg("plugins.reddit.remove-subreddit-error-not-found")
		return r.actionFinish
	}
	helpers.Relax(err)

	err = helpers.MDbDelete(models.RedditSubredditsTable, subredditEntry.ID)
	helpers.Relax(err)

	_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(subredditEntry.ID),
		models.EventlogTargetTypeRobyulRedditFeed, in.Author.ID,
		models.EventlogTypeRobyulRedditFeedRemove, "",
		nil,
		[]models.ElasticEventlogOption{
			{
				Key:   "reddit_channelid",
				Value: subredditEntry.ChannelID,
				Type:  models.EventlogTargetTypeChannel,
			},
			{
				Key:   "reddit_postdirectlinks",
				Value: helpers.StoreBoolAsString(subredditEntry.PostDirectLinks),
			},
			{
				Key:   "reddit_postdelay",
				Value: strconv.Itoa(subredditEntry.PostDelay),
			},
			{
				Key:   "reddit_subredditname",
				Value: subredditEntry.SubredditName,
			},
		}, false)
	helpers.RelaxLog(err)

	*out = r.newMsg("plugins.reddit.remove-subreddit-success", subredditEntry.SubredditName)
	return r.actionFinish
}

func (r *Reddit) actionInfo(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	searchName := strings.TrimLeft(args[0], "/")

	if strings.HasPrefix(searchName, "u/") {
		searchName = strings.Replace(searchName, "u/", "", -1)

		*out = r.getRedditorInfo(searchName)
		return r.actionFinish
	}

	searchName = strings.Replace(searchName, "r/", "", -1)

	*out = r.getSubredditInfo(searchName)
	return r.actionFinish
}

func (r *Reddit) actionToggleDirectLinks(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	if !helpers.IsMod(in) {
		*out = r.newMsg(helpers.GetText("mod.no_permission"))
		return r.actionFinish
	}

	if len(args) < 2 {
		*out = r.newMsg("bot.arguments.too-few")
		return r.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	var subredditEntry models.RedditSubredditEntry
	err = helpers.MdbOne(
		helpers.MdbCollection(models.RedditSubredditsTable).Find(bson.M{"guildid": channel.GuildID, "_id": helpers.HumanToMdbId(args[1])}),
		&subredditEntry,
	)
	if helpers.IsMdbNotFound(err) {
		*out = r.newMsg("plugins.reddit.toggledirectlinks-error-subreddit-not-found")
		return r.actionFinish
	}
	helpers.Relax(err)

	beforeValue := subredditEntry.PostDirectLinks

	if subredditEntry.PostDirectLinks {
		subredditEntry.PostDirectLinks = false
		*out = r.newMsg("plugins.reddit.toggledirectlinks-disabled", subredditEntry.SubredditName)
	} else {
		subredditEntry.PostDirectLinks = true
		*out = r.newMsg("plugins.reddit.toggledirectlinks-enabled", subredditEntry.SubredditName)
	}

	_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(subredditEntry.ID),
		models.EventlogTargetTypeRobyulRedditFeed, in.Author.ID,
		models.EventlogTypeRobyulRedditFeedUpdate, "",
		[]models.ElasticEventlogChange{
			{
				Key:      "reddit_postdirectlinks",
				OldValue: helpers.StoreBoolAsString(beforeValue),
				NewValue: helpers.StoreBoolAsString(subredditEntry.PostDirectLinks),
			},
		},
		[]models.ElasticEventlogOption{
			{
				Key:   "reddit_channelid",
				Value: subredditEntry.ChannelID,
				Type:  models.EventlogTargetTypeChannel,
			},
			{
				Key:   "reddit_postdirectlinks",
				Value: helpers.StoreBoolAsString(subredditEntry.PostDirectLinks),
			},
			{
				Key:   "reddit_postdelay",
				Value: strconv.Itoa(subredditEntry.PostDelay),
			},
			{
				Key:   "reddit_subredditname",
				Value: subredditEntry.SubredditName,
			},
		}, false)
	helpers.RelaxLog(err)

	err = helpers.MDbUpdate(models.RedditSubredditsTable, subredditEntry.ID, subredditEntry)
	helpers.Relax(err)

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
	titleText := "/r/" + subredditData.Name
	if subredditData.Title != "" {
		titleText += " ~ " + subredditData.Title
	}

	creationTime := time.Unix(int64(subredditData.DateCreated), 0)

	data.Embed = &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.reddit.embed-footer") + " | reddit #" + subredditData.ID,
			IconURL: helpers.GetText("plugins.reddit.embed-footer-imageurl"),
		},
		Title:       html.UnescapeString(titleText),
		Description: html.UnescapeString(subredditData.PublicDesc),
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

func (r *Reddit) getRedditorInfo(username string) (data *discordgo.MessageSend) {
	redditorData, err := redditSession.AboutRedditor(username)
	if err != nil {
		return r.newMsg(err.Error())
	}

	if redditorData.ID == "" {
		return r.newMsg("plugins.reddit.redditor-not-found")
	}

	data = &discordgo.MessageSend{}

	hasGold := "No"
	if redditorData.Gold {
		hasGold = "Yes"
	}

	titleText := "/u/" + redditorData.Name

	creationTime := time.Unix(int64(redditorData.Created), 0)

	data.Embed = &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.reddit.embed-footer") + " | reddit #" + redditorData.ID,
			IconURL: helpers.GetText("plugins.reddit.embed-footer-imageurl"),
		},
		Title: html.UnescapeString(titleText),
		URL:   RedditBaseUrl + "/u/" + redditorData.Name,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Karma (Post / Comment)",
				Value:  humanize.Comma(int64(redditorData.LinkKarma)) + " / " + humanize.Comma(int64(redditorData.CommentKarma)),
				Inline: true},
			{Name: "Creation", Value: fmt.Sprintf("%s. That's %s.",
				creationTime.Format(time.ANSIC), helpers.SinceInDaysText(creationTime)), Inline: true},
			{Name: "Gold User?", Value: hasGold, Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(RedditColor),
	}

	return
}

func (r *Reddit) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) redditAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.RelaxMessage(err, in.ChannelID, in.ID)

	return nil
}

func (r *Reddit) newMsg(content string, replacements ...interface{}) *discordgo.MessageSend {
	if len(replacements) < 1 {
		return &discordgo.MessageSend{Content: helpers.GetText(content)}
	}
	return &discordgo.MessageSend{Content: helpers.GetTextF(content, replacements...)}
}

func (r *Reddit) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (r *Reddit) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "reddit")
}
