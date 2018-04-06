package levels

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"io/ioutil"
	"math"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/ratelimits"
	"github.com/Seklfreak/lastfm-go/lastfm"
	"github.com/andybons/gogif"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	rethink "github.com/gorethink/gorethink"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/nfnt/resize"
	"gopkg.in/oleiade/lane.v1"
)

type Levels struct {
	sync.RWMutex

	buckets map[string]int8
}

type ProcessExpInfo struct {
	GuildID   string
	ChannelID string
	UserID    string
}

var (
	LevelsBucket = &ratelimits.BucketContainer{}

	// How many keys a bucket may contain when created
	BUCKET_INITIAL_FILL int8 = 1

	// The maximum amount of keys a user may possess
	BUCKET_UPPER_BOUND int8 = 1

	// How often new keys drip into the buckets
	DROP_INTERVAL = 60 * time.Second

	// How many keys may drop at a time
	DROP_SIZE int8 = 1

	temporaryIgnoredGuilds []string

	expStack = lane.NewStack()
)

func (m *Levels) Commands() []string {
	return []string{
		"level",
		"levels",
		"profile",
		"rep",
		"gif-profile",
		"leaderboard",
		"leaderboards",
		"ranking",
		"rankings",
	}
}

type Cache_Levels_top struct {
	GuildID string
	Levels  PairList
}

type Levels_Cache_Ranking_Item struct {
	UserID  string
	EXP     int64
	Level   int
	Ranking int
}

var (
	cachePath                string
	assetsPath               string
	htmlTemplateString       string
	levelsEnv                = os.Environ()
	topCache                 []Cache_Levels_top
	activeBadgePickerUserIDs map[string]string
	repCommandLocks          = make(map[string]*sync.Mutex)
)

const (
	BadgeLimt          = 18
	TimeAtUserFormat   = "Mon, 15:04"
	TimeBirthdayFormat = "01/02"
)

func (m *Levels) Init(session *discordgo.Session) {
	m.BucketInit()

	log := cache.GetLogger()

	cachePath = helpers.GetConfig().Path("cache_folder").Data().(string)
	assetsPath = helpers.GetConfig().Path("assets_folder").Data().(string)
	htmlTemplate, err := ioutil.ReadFile(assetsPath + "profile.html")
	helpers.Relax(err)
	htmlTemplateString = string(htmlTemplate)

	go processExpStackLoop()
	log.WithField("module", "levels").Info("Started processExpStackLoop")

	go cacheTopLoop()
	log.WithField("module", "levels").Info("Started processCacheTopLoop")

	activeBadgePickerUserIDs = make(map[string]string, 0)

	go setServerFeaturesLoop()
}

func (l *Levels) Uninit(session *discordgo.Session) {

}

func (m *Levels) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermLevels) {
		return
	}

	switch command {
	case "rep": // [p]rep <user id/mention>
		session.ChannelTyping(msg.ChannelID)
		args := strings.Fields(content)

		m.lockRepUser(msg.Author.ID)
		defer m.unlockRepUser(msg.Author.ID)

		userData, err := m.GetUserUserdata(msg.Author)
		helpers.Relax(err)

		if len(args) <= 0 {
			if time.Since(userData.LastRepped).Hours() < 12 {
				timeUntil := time.Until(userData.LastRepped.Add(time.Hour * 12))
				if timeUntil.Minutes() < 1 {
					helpers.SendMessage(msg.ChannelID,
						helpers.GetTextF("plugins.levels.rep-next-rep-seconds", int(math.Floor(timeUntil.Seconds()))))
				} else {
					helpers.SendMessage(msg.ChannelID,
						helpers.GetTextF("plugins.levels.rep-next-rep",
							int(math.Floor(timeUntil.Hours())),
							int(math.Floor(timeUntil.Minutes()))-(int(math.Floor(timeUntil.Hours()))*60)))
				}
			} else {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.rep-target"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			}
			return
		}

		if time.Since(userData.LastRepped).Hours() < 12 {
			timeUntil := time.Until(userData.LastRepped.Add(time.Hour * 12))
			if timeUntil.Minutes() < 1 {
				helpers.SendMessage(msg.ChannelID,
					helpers.GetTextF("plugins.levels.rep-error-timelimit-seconds", int(math.Floor(timeUntil.Seconds()))))
			} else {
				helpers.SendMessage(msg.ChannelID,
					helpers.GetTextF("plugins.levels.rep-error-timelimit",
						int(math.Floor(timeUntil.Hours())),
						int(math.Floor(timeUntil.Minutes()))-(int(math.Floor(timeUntil.Hours()))*60)))
			}
			return
		}

		targetUser, err := helpers.GetUserFromMention(args[0])
		if err != nil || targetUser == nil || targetUser.ID == "" {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}

		// Don't rep this bot account, other bots, or oneself
		if targetUser.ID == session.State.User.ID {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.rep-error-session"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}
		if targetUser.ID == msg.Author.ID {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.rep-error-self"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}
		if targetUser.Bot == true {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.rep-error-bot"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}

		targetUserData, err := m.GetUserUserdata(targetUser)
		helpers.Relax(err)
		targetUserData.Rep += 1
		err = helpers.MDbUpdate(models.ProfileUserdataTable, targetUserData.ID, targetUserData)
		helpers.Relax(err)

		userData.LastRepped = time.Now()
		err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
		helpers.Relax(err)

		_, err = helpers.SendMessage(msg.ChannelID,
			helpers.GetTextF("plugins.levels.rep-success", targetUser.Username))
		helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		return
	case "profile", "gif-profile": // [p]profile
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)
		if _, ok := activeBadgePickerUserIDs[msg.Author.ID]; ok {
			if activeBadgePickerUserIDs[msg.Author.ID] != msg.ChannelID {
				_, err := helpers.SendMessage(
					msg.ChannelID, helpers.GetTextF("plugins.levels.badge-picker-session-duplicate", helpers.GetPrefixForServer(channel.GuildID)))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			}
			return
		}
		session.ChannelTyping(msg.ChannelID)
		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)
		targetUser, err := helpers.GetUser(msg.Author.ID)
		helpers.Relax(err)
		args := strings.Fields(content)
		if len(args) >= 1 && args[0] != "" {
			switch args[0] {
			case "toggle-lastfm":
				userUserdata, err := m.GetUserUserdata(msg.Author)
				helpers.Relax(err)

				var message string
				if userUserdata.HideLastFm {
					userUserdata.HideLastFm = false
					message = helpers.GetText("plugins.levels.profile-lastfm-shown")
				} else {
					userUserdata.HideLastFm = true
					message = helpers.GetText("plugins.levels.profile-lastfm-hidden")
				}
				err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, message)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "title":
				titleText := " "
				if len(args) >= 2 {
					titleText = strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
				}

				userUserdata, err := m.GetUserUserdata(msg.Author)
				helpers.Relax(err)
				userUserdata.Title = titleText
				err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-title-set-success"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "bio":
				bioText := " "
				if len(args) >= 2 {
					bioText = strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
				}

				userUserdata, err := m.GetUserUserdata(msg.Author)

				oldBioText := userUserdata.Bio

				userUserdata.Bio = bioText
				err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
				helpers.Relax(err)

				message := helpers.GetText("plugins.levels.profile-bio-set-success")
				if oldBioText != "" && oldBioText != " " && bioText == " " {
					message = helpers.GetTextF("plugins.levels.profile-bio-reset-success", oldBioText)
				}

				_, err = helpers.SendMessage(msg.ChannelID, message)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "background", "backgrounds":
				if len(args) < 2 {
					if len(msg.Attachments) <= 0 {
						userUserdata, err := m.GetUserUserdata(msg.Author)

						if userUserdata.Background != "" {
							helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.new-profile-background-help-withbackground", userUserdata.Background))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}
						if userUserdata.BackgroundObjectName != "" {
							helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.new-profile-background-help-withbackground", m.GetProfileBackgroundUrl(userUserdata)))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.new-profile-background-help"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}

					quitChannel := helpers.StartTypingLoop(msg.ChannelID)
					defer func() { quitChannel <- 0 }()

					if helpers.UseruploadsIsDisabled(msg.Author.ID) {
						quitChannel <- 0
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.useruploads-disabled"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}

					// <= 2 MB, 400x300px?
					if msg.Attachments[0].Size > 2e+6 || msg.Attachments[0].Width < 400 || msg.Attachments[0].Height < 300 {
						quitChannel <- 0
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-background-wrong-dimensions"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}

					bytesData, err := helpers.NetGetUAWithErrorAndTimeout(msg.Attachments[0].URL, helpers.DEFAULT_UA, time.Second*15)
					helpers.Relax(err)

					imageConfig, _, err := image.DecodeConfig(bytes.NewReader(bytesData))
					helpers.Relax(err)

					// check 400x300px again on Robyul
					if imageConfig.Width < 400 || imageConfig.Height < 300 {
						quitChannel <- 0
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-background-wrong-dimensions"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}

					// scales image if it is too big
					if imageConfig.Width > 400 || imageConfig.Height > 300 {
						bytesData, err = helpers.ScaleImage(bytesData, 400, 300)
						helpers.Relax(err)
					}

					metrics.CloudVisionApiRequests.Add(1)
					if !helpers.PictureIsSafe(bytes.NewReader(bytesData)) {
						quitChannel <- 0

						go func() {
							logChannelID, _ := helpers.GetBotConfigString(models.UserProfileBackgroundLogChannelKey)
							if logChannelID != "" {
								err = m.logUserBackgroundNotSafe(logChannelID, msg.ChannelID, msg.Author.ID, msg.Attachments[0].URL)
								helpers.RelaxLog(err)
							}
						}()
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-background-not-safe"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}

					backgroundObjectName, err := helpers.AddFile("", bytesData, helpers.AddFileMetadata{
						ChannelID: msg.ChannelID,
						UserID:    msg.Author.ID,
					}, "levels", true)
					if err != nil {
						helpers.RelaxLog(err)
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-background-upload-failed"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}

					userUserdata, err := m.GetUserUserdata(msg.Author)
					helpers.Relax(err)

					// delete old background object if set
					if userUserdata.BackgroundObjectName != "" {
						err = helpers.DeleteFile(userUserdata.BackgroundObjectName)
					}

					userUserdata.Background = ""
					userUserdata.BackgroundObjectName = backgroundObjectName
					err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
					helpers.Relax(err)

					go func() {
						logChannelID, _ := helpers.GetBotConfigString(models.UserProfileBackgroundLogChannelKey)
						if logChannelID != "" {
							backgroundUrl, err := helpers.GetFileLink(userUserdata.BackgroundObjectName)
							helpers.RelaxLog(err)
							if err == nil {
								err = m.logUserBackgroundSet(logChannelID, msg.ChannelID, msg.Author.ID, backgroundUrl)
								helpers.RelaxLog(err)
							}
						}
					}()

					quitChannel <- 0
					_, err = helpers.SendMessage(msg.ChannelID,
						helpers.GetTextF("plugins.levels.user-background-success",
							helpers.GetPrefixForServer(channel.GuildID)))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				switch args[1] {
				// [p]profile background force <@user or user id> <<image url>|+ ATTACHMENT>
				case "force":
					helpers.RequireRobyulMod(msg, func() {
						if !((len(args) >= 3 && len(msg.Attachments) > 0) || len(args) >= 4) {
							helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							return
						}

						sourceUrl := args[len(args)-1]
						if len(msg.Attachments) > 0 {
							sourceUrl = msg.Attachments[0].URL
						}

						userToChange, err := helpers.GetUserFromMention(args[2])
						helpers.Relax(err)

						bytesData, err := helpers.NetGetUAWithErrorAndTimeout(sourceUrl, helpers.DEFAULT_UA, time.Second*15)
						helpers.Relax(err)

						imageConfig, _, err := image.DecodeConfig(bytes.NewReader(bytesData))
						helpers.Relax(err)

						// reject smaller than 400x300px
						if imageConfig.Width < 400 || imageConfig.Height < 300 {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-background-wrong-dimensions"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						// scales image if it is too big
						if imageConfig.Width > 400 || imageConfig.Height > 300 {
							bytesData, err = helpers.ScaleImage(bytesData, 400, 300)
							helpers.Relax(err)
						}

						backgroundObjectName, err := helpers.AddFile("", bytesData, helpers.AddFileMetadata{
							ChannelID: msg.ChannelID,
							UserID:    msg.Author.ID,
						}, "levels", true)
						if err != nil {
							helpers.RelaxLog(err)
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-background-upload-failed"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						userUserdata, err := m.GetUserUserdata(userToChange)
						helpers.Relax(err)

						// delete old background object if set
						if userUserdata.BackgroundObjectName != "" {
							err = helpers.DeleteFile(userUserdata.BackgroundObjectName)
						}

						userUserdata.Background = ""
						userUserdata.BackgroundObjectName = backgroundObjectName
						err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
						helpers.Relax(err)

						_, err = helpers.SendMessage(msg.ChannelID,
							helpers.GetTextF("plugins.levels.user-force-background-success",
								userToChange.Username))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					})
					return
				case "reset":
					helpers.RequireRobyulMod(msg, func() {
						if len(args) < 3 {
							helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							return
						}

						userToReset, err := helpers.GetUserFromMention(args[2])
						helpers.Relax(err)

						userUserdata, err := m.GetUserUserdata(userToReset)
						helpers.Relax(err)
						userUserdata.Background = ""
						err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
						helpers.Relax(err)

						_, err = helpers.SendMessage(msg.ChannelID,
							helpers.GetTextF("plugins.levels.user-reset-success",
								userToReset.Username))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					})
					return
				case "set-log":
					helpers.RequireRobyulMod(msg, func() {
						var err error
						var targetChannel *discordgo.Channel

						if len(args) >= 3 {
							targetChannel, err = helpers.GetChannelFromMention(msg, args[2])
							helpers.Relax(err)
						}

						if targetChannel != nil && targetChannel.ID != "" {
							err = helpers.SetBotConfigString(models.UserProfileBackgroundLogChannelKey, targetChannel.ID)
						} else {
							err = helpers.SetBotConfigString(models.UserProfileBackgroundLogChannelKey, "")
						}

						_, err = helpers.SendMessage(msg.ChannelID,
							helpers.GetText("plugins.levels.background-setlog-success"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					})
					return
				case "add":
					helpers.RequireRobyulMod(msg, func() {
						if len(args) < 5 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}
						backgroundName := args[2]
						backgroundUrl := args[3]

						tagsText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:4], " "), "", 1))
						tags := make([]string, 0)

						for _, tag := range strings.Split(tagsText, ",") {
							tag = strings.ToLower(strings.TrimSpace(tag))
							alreadyInList := false
							for _, oldTag := range tags {
								if oldTag == tag {
									alreadyInList = true
								}
							}
							if alreadyInList == false {
								tags = append(tags, tag)
							}
						}

						if len(tags) <= 0 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						if helpers.UseruploadsIsDisabled(msg.Author.ID) {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.useruploads-disabled"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						picData, err := helpers.NetGetUAWithError(backgroundUrl, helpers.DEFAULT_UA)
						if err != nil {
							if _, ok := err.(*url.Error); ok {
								_, err = helpers.SendMessage(msg.ChannelID, "Invalid url.")
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							} else {
								helpers.Relax(err)
							}
							return
						}
						objectName, err := helpers.AddFile("", picData, helpers.AddFileMetadata{
							ChannelID: msg.ChannelID,
							UserID:    msg.Author.ID,
						}, "levels", true)
						helpers.Relax(err)

						var entryBucket models.ProfileBackgroundEntry
						err = helpers.MdbOne(
							helpers.MdbCollection(models.ProfileBackgroundsTable).Find(bson.M{"name": strings.ToLower(backgroundName)}),
							&entryBucket,
						)
						if !helpers.IsMdbNotFound(err) {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.new-profile-background-add-error-duplicate"))
							return
						}

						_, err = helpers.MDbInsert(
							models.ProfileBackgroundsTable,
							models.ProfileBackgroundEntry{
								Name:       strings.ToLower(backgroundName),
								ObjectName: objectName,
								CreatedAt:  time.Now(),
								Tags:       tags,
							},
						)
						helpers.Relax(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.new-profile-background-add-success",
							backgroundName, strings.Join(tags, ", ")))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					})
					return
				case "delete":
					helpers.RequireRobyulMod(msg, func() {
						if len(args) < 3 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}
						backgroundName := strings.ToLower(args[2])

						var entryBucket models.ProfileBackgroundEntry
						err = helpers.MdbOne(
							helpers.MdbCollection(models.ProfileBackgroundsTable).Find(bson.M{"name": strings.ToLower(backgroundName)}),
							&entryBucket,
						)
						if helpers.IsMdbNotFound(err) {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-delete-error-not-found"))
							return
						}
						backgroundUrl := m.GetProfileBackgroundUrlByName(backgroundName)

						if helpers.ConfirmEmbed(
							msg.ChannelID, msg.Author, helpers.GetTextF("plugins.levels.profile-background-delete-confirm",
								backgroundName, backgroundUrl),
							"✅", "🚫") == true {
							err = helpers.MDbDelete(models.ProfileBackgroundsTable, entryBucket.ID)
							helpers.Relax(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-delete-success"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						}
						return
					})
					return
				default:
					var entryBucket models.ProfileBackgroundEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.ProfileBackgroundsTable).Find(bson.M{"name": strings.ToLower(args[1])}),
						&entryBucket,
					)
					if helpers.IsMdbNotFound(err) {
						searchResult := m.ProfileBackgroundSearch(args[1])

						if len(searchResult) <= 0 {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-set-error-not-found"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						} else {
							backgroundNamesText := ""
							for _, entry := range searchResult {
								backgroundNamesText += "`" + entry.Name + "` "
							}
							backgroundNamesText = strings.TrimSpace(backgroundNamesText)
							resultText := helpers.GetText("plugins.levels.profile-background-set-error-not-found") + "\n"
							resultText += fmt.Sprintf("Maybe I can interest you in one of these backgrounds: %s", backgroundNamesText)

							_, err = helpers.SendMessage(msg.ChannelID, resultText)
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						}
						return
					}

					userUserdata, err := m.GetUserUserdata(msg.Author)
					helpers.Relax(err)

					// delete old background object if set
					if userUserdata.BackgroundObjectName != "" {
						err = helpers.DeleteFile(userUserdata.BackgroundObjectName)
					}

					userUserdata.Background = args[1]
					userUserdata.BackgroundObjectName = ""
					err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
					helpers.Relax(err)

					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-set-success"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
			case "badge", "badges":
				if len(args) >= 2 {
					switch args[1] {
					case "create": // [p]profile badge create <category name> <badge name> <image url> <border color> <level req, -1=not available, 0=everyone> [global, botowner only]
						helpers.RequireAdmin(msg, func() {
							session.ChannelTyping(msg.ChannelID)
							if len(args) < 7 {
								helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
								return
							}

							if helpers.UseruploadsIsDisabled(msg.Author.ID) {
								_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.useruploads-disabled"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							channel, err := helpers.GetChannel(msg.ChannelID)
							helpers.Relax(err)

							guild, err := helpers.GetGuild(channel.GuildID)
							helpers.Relax(err)

							badgeData, err := helpers.NetGetUAWithError(args[4], helpers.DEFAULT_UA)
							helpers.Relax(err)

							badgeData, err = helpers.ScaleImage(badgeData, 28, 28)
							helpers.Relax(err)

							objectName, err := helpers.AddFile("", badgeData, helpers.AddFileMetadata{
								Filename:  "",
								ChannelID: msg.ChannelID,
								UserID:    msg.Author.ID,
							}, "levels", true)
							if err != nil {
								if _, ok := err.(*url.Error); ok {
									helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
									return
								}
							}
							helpers.Relax(err)

							newBadge := new(models.ProfileBadgeEntry)
							newBadge.CreatedByUserID = msg.Author.ID
							newBadge.GuildID = channel.GuildID
							newBadge.CreatedAt = time.Now()
							newBadge.Category = strings.ToLower(args[2])
							newBadge.Name = strings.ToLower(args[3])
							newBadge.ObjectName = objectName
							newBadge.BorderColor = strings.Replace(args[5], "#", "", -1) // check if valid color
							newBadge.LevelRequirement, err = strconv.Atoi(args[6])
							if err != nil {
								if args[6] == "role" && len(args) >= 8 {
									// trying to connect badge to role
									roleToMatch := strings.ToLower(args[7])
									var matchedRole *discordgo.Role
									for _, role := range guild.Roles {
										if role.ID == roleToMatch || strings.ToLower(role.Name) == roleToMatch {
											matchedRole = role
										}
									}

									if matchedRole == nil || matchedRole.ID == "" {
										_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
										helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
										return
									}

									newBadge.RoleRequirement = matchedRole.ID
								} else {
									_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
									helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
									return
								}
							} else {
								if len(args) >= 8 {
									if args[7] == "global" {
										if helpers.IsBotAdmin(msg.Author.ID) {
											newBadge.GuildID = "global"
										} else {
											_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
											helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
											return
										}
									}
								}
							}

							badgeFound := getBadge(newBadge.Category, newBadge.Name, channel.GuildID)
							if badgeFound.ID != "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.create-badge-error-duplicate"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							serverBadges := getServerOnlyBadges(channel.GuildID)
							badgeLimit := helpers.GetMaxBadgesForGuild(channel.GuildID)
							if len(serverBadges) >= badgeLimit {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.create-badge-error-too-many", helpers.GetStaffUsernamesText()))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							badgeID, err := helpers.MDbInsert(
								models.ProfileBadgesTable,
								newBadge,
							)
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(badgeID),
								models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
								models.EventlogTypeRobyulBadgeCreate, "", nil,
								[]models.ElasticEventlogOption{
									{
										Key:   "badge_category",
										Value: newBadge.Category,
									},
									{
										Key:   "badge_name",
										Value: newBadge.Name,
									},
									{
										Key:   "badge_url",
										Value: getBadgeUrl(*newBadge),
									},
									{
										Key:   "badge_bordercolor",
										Value: newBadge.BorderColor,
									},
									{
										Key:   "badge_levelrequirement",
										Value: strconv.Itoa(newBadge.LevelRequirement),
									},
									{
										Key:   "badge_rolerequirement",
										Value: newBadge.RoleRequirement,
										Type:  models.EventlogTargetTypeRole,
									},
									{
										Key:   "badge_guildid",
										Value: newBadge.GuildID,
										Type:  models.EventlogTargetTypeGuild,
									},
									{
										Key:   "badge_alloweduserids",
										Value: strings.Join(newBadge.AllowedUserIDs, ","),
										Type:  models.EventlogTargetTypeUser,
									},
									{
										Key:   "badge_denieduserids",
										Value: strings.Join(newBadge.DeniedUserIDs, ","),
										Type:  models.EventlogTargetTypeUser,
									},
								}, false)
							helpers.RelaxLog(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.create-badge-success"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						})
						return
					case "delete", "remove": // [p]profile badge delete <category name> <badge name>
						helpers.RequireAdmin(msg, func() {
							session.ChannelTyping(msg.ChannelID)
							if len(args) < 4 {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}
							channel, err := helpers.GetChannel(msg.ChannelID)
							helpers.Relax(err)

							badgeFound := getBadge(args[2], args[3], channel.GuildID)
							if badgeFound.ID == "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-not-found"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}
							if badgeFound.GuildID == "global" && !helpers.IsBotAdmin(msg.Author.ID) {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.delete-badge-error-not-allowed"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							err = helpers.MDbDelete(models.ProfileBadgesTable, badgeFound.ID)
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(badgeFound.ID),
								models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
								models.EventlogTypeRobyulBadgeDelete, "", nil,
								[]models.ElasticEventlogOption{
									{
										Key:   "badge_category",
										Value: badgeFound.Category,
									},
									{
										Key:   "badge_name",
										Value: badgeFound.Name,
									},
									{
										Key:   "badge_url",
										Value: getBadgeUrl(badgeFound),
									},
									{
										Key:   "badge_bordercolor",
										Value: badgeFound.BorderColor,
									},
									{
										Key:   "badge_levelrequirement",
										Value: strconv.Itoa(badgeFound.LevelRequirement),
									},
									{
										Key:   "badge_rolerequirement",
										Value: badgeFound.RoleRequirement,
										Type:  models.EventlogTargetTypeRole,
									},
									{
										Key:   "badge_guildid",
										Value: badgeFound.GuildID,
										Type:  models.EventlogTargetTypeGuild,
									},
									{
										Key:   "badge_alloweduserids",
										Value: strings.Join(badgeFound.AllowedUserIDs, ","),
										Type:  models.EventlogTargetTypeUser,
									},
									{
										Key:   "badge_denieduserids",
										Value: strings.Join(badgeFound.DeniedUserIDs, ","),
										Type:  models.EventlogTargetTypeUser,
									},
								}, false)
							helpers.RelaxLog(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.delete-badge-success"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						})
						return
					case "list": // [p]profile badge list [<category name>]
						session.ChannelTyping(msg.ChannelID)
						if len(args) >= 3 {
							categoryName := args[2]

							channel, err := helpers.GetChannel(msg.ChannelID)
							helpers.Relax(err)

							categoryBadges := getCategoryBadges(categoryName, channel.GuildID)

							if categoryBadges == nil || len(categoryBadges) <= 0 {
								helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.list-category-badge-error-none"))
								return
							}

							slice.Sort(categoryBadges, func(i, j int) bool {
								return categoryBadges[i].Name < categoryBadges[j].Name
							})

							resultText := fmt.Sprintf("__**Badges in %s**__\n", categoryName)
							for _, badge := range categoryBadges {
								globalText := ""
								if badge.GuildID == "global" {
									globalText = "GLOBAL "
								}

								var requirementText string
								if badge.RoleRequirement != "" {
									requirementRole, err := session.State.Role(badge.GuildID, badge.RoleRequirement)
									if err == nil {
										requirementText = fmt.Sprintf("Role: %s (`#%s`)", requirementRole.Name, requirementRole.ID)
									} else {
										requirementText = fmt.Sprintf("Role: N/A (`#%s`)", badge.RoleRequirement)
									}
								} else {
									requirementText = "Level >= " + strconv.Itoa(badge.LevelRequirement)
								}

								resultText += fmt.Sprintf("**%s%s**: URL: <%s>, Border Color: #%s, Requirement: %s, Allowed Users: %d, Denied Users %d\n",
									globalText, badge.Name, getBadgeUrl(badge), badge.BorderColor, requirementText, len(badge.AllowedUserIDs), len(badge.DeniedUserIDs),
								)
							}
							resultText += fmt.Sprintf("I found %d badges in this category.\n",
								len(categoryBadges))

							_, err = helpers.SendMessage(msg.ChannelID, resultText)
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						channel, err := helpers.GetChannel(msg.ChannelID)
						helpers.Relax(err)

						serverBadges := getServerBadges(channel.GuildID)

						if len(serverBadges) <= 0 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.list-badge-error-none"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						categoriesCount := make(map[string]int, 0)
						for _, badge := range serverBadges {
							if _, ok := categoriesCount[badge.Category]; ok {
								categoriesCount[badge.Category] += 1
							} else {
								categoriesCount[badge.Category] = 1
							}
						}

						sortedKeys := make([]string, len(categoriesCount))
						i := 0
						for k := range categoriesCount {
							sortedKeys[i] = k
							i++
						}
						sort.Strings(sortedKeys)

						resultText := "__**Badge Categories**__\n"
						for _, key := range sortedKeys {
							resultText += fmt.Sprintf("**%s** (%d badges)\n", key, categoriesCount[key])
						}
						resultText += fmt.Sprintf("I found %d badge categories on this server.\nUse `_profile badge list <category name>` to view all badges of a category.\n",
							len(categoriesCount))

						for _, page := range helpers.Pagify(resultText, "\n") {
							_, err = helpers.SendMessage(msg.ChannelID, page)
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						}
						return
					case "allow": // [p]profile badge allow <user id/mention> <category name> <badge name>
						helpers.RequireMod(msg, func() {
							session.ChannelTyping(msg.ChannelID)
							if len(args) < 5 {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							targetUser, err := helpers.GetUserFromMention(args[2])
							if err != nil || targetUser.ID == "" {
								helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
								return
							}

							channel, err := helpers.GetChannel(msg.ChannelID)
							helpers.Relax(err)

							badgeToAllow := getBadge(args[3], args[4], channel.GuildID)
							if badgeToAllow.ID == "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-not-found"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							if badgeToAllow.GuildID == "global" && !helpers.IsBotAdmin(msg.Author.ID) {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.edit-badge-error-not-allowed"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							isAlreadyAllowed := false
							for _, userAllowedID := range badgeToAllow.AllowedUserIDs {
								if userAllowedID == targetUser.ID {
									isAlreadyAllowed = true
								}
							}

							allowedIDsBefore := badgeToAllow.AllowedUserIDs
							if isAlreadyAllowed == false {
								badgeToAllow.AllowedUserIDs = append(badgeToAllow.AllowedUserIDs, targetUser.ID)
								err = helpers.MDbUpdate(models.ProfileBadgesTable, badgeToAllow.ID, badgeToAllow)
								helpers.Relax(err)

								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(badgeToAllow.ID),
									models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
									models.EventlogTypeRobyulBadgeAllow, "",
									[]models.ElasticEventlogChange{
										{
											Key:      "badge_alloweduserids",
											OldValue: strings.Join(allowedIDsBefore, ","),
											NewValue: strings.Join(badgeToAllow.AllowedUserIDs, ","),
											Type:     models.EventlogTargetTypeUser,
										},
									},
									[]models.ElasticEventlogOption{
										{
											Key:   "badge_category",
											Value: badgeToAllow.Category,
										},
										{
											Key:   "badge_name",
											Value: badgeToAllow.Name,
										},
										{
											Key:   "badge_url",
											Value: getBadgeUrl(badgeToAllow),
										},
										{
											Key:   "badge_bordercolor",
											Value: badgeToAllow.BorderColor,
										},
										{
											Key:   "badge_levelrequirement",
											Value: strconv.Itoa(badgeToAllow.LevelRequirement),
										},
										{
											Key:   "badge_rolerequirement",
											Value: badgeToAllow.RoleRequirement,
											Type:  models.EventlogTargetTypeRole,
										},
										{
											Key:   "badge_guildid",
											Value: badgeToAllow.GuildID,
											Type:  models.EventlogTargetTypeGuild,
										},
										{
											Key:   "badge_alloweduserids_added",
											Value: targetUser.ID,
											Type:  models.EventlogTargetTypeUser,
										},
									}, false)
								helpers.RelaxLog(err)

								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.allow-badge-success-allowed",
									targetUser.Username, badgeToAllow.Name, badgeToAllow.Category))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							} else {
								allowedUserIDsWithout := make([]string, 0)
								for _, userAllowedID := range badgeToAllow.AllowedUserIDs {
									if userAllowedID != targetUser.ID {
										allowedUserIDsWithout = append(allowedUserIDsWithout, userAllowedID)
									}
								}
								badgeToAllow.AllowedUserIDs = allowedUserIDsWithout
								err = helpers.MDbUpdate(models.ProfileBadgesTable, badgeToAllow.ID, badgeToAllow)
								helpers.Relax(err)

								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(badgeToAllow.ID),
									models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
									models.EventlogTypeRobyulBadgeAllow, "",
									[]models.ElasticEventlogChange{
										{
											Key:      "badge_alloweduserids",
											OldValue: strings.Join(allowedIDsBefore, ","),
											NewValue: strings.Join(badgeToAllow.AllowedUserIDs, ","),
											Type:     models.EventlogTargetTypeUser,
										},
									},
									[]models.ElasticEventlogOption{
										{
											Key:   "badge_category",
											Value: badgeToAllow.Category,
										},
										{
											Key:   "badge_name",
											Value: badgeToAllow.Name,
										},
										{
											Key:   "badge_url",
											Value: getBadgeUrl(badgeToAllow),
										},
										{
											Key:   "badge_bordercolor",
											Value: badgeToAllow.BorderColor,
										},
										{
											Key:   "badge_levelrequirement",
											Value: strconv.Itoa(badgeToAllow.LevelRequirement),
										},
										{
											Key:   "badge_rolerequirement",
											Value: badgeToAllow.RoleRequirement,
											Type:  models.EventlogTargetTypeRole,
										},
										{
											Key:   "badge_guildid",
											Value: badgeToAllow.GuildID,
											Type:  models.EventlogTargetTypeGuild,
										},
										{
											Key:   "badge_alloweduserids_removed",
											Value: targetUser.ID,
											Type:  models.EventlogTargetTypeUser,
										},
									}, false)
								helpers.RelaxLog(err)

								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.allow-badge-success-not-allowed",
									targetUser.Username, badgeToAllow.Name, badgeToAllow.Category))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}
						})
						return
					case "deny": // [p]profile badge deny <user id/mention> <category name> <badge name>
						helpers.RequireMod(msg, func() {
							session.ChannelTyping(msg.ChannelID)
							if len(args) < 5 {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							targetUser, err := helpers.GetUserFromMention(args[2])
							if err != nil || targetUser.ID == "" {
								helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
								return
							}

							channel, err := helpers.GetChannel(msg.ChannelID)
							helpers.Relax(err)

							badgeToDeny := getBadge(args[3], args[4], channel.GuildID)
							if badgeToDeny.ID == "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-not-found"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							if badgeToDeny.GuildID == "global" && !helpers.IsBotAdmin(msg.Author.ID) {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.edit-badge-error-not-allowed"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							isAlreadyDenied := false
							for _, userDeniedID := range badgeToDeny.DeniedUserIDs {
								if userDeniedID == targetUser.ID {
									isAlreadyDenied = true
								}
							}

							deniedIDsBefore := badgeToDeny.DeniedUserIDs
							if isAlreadyDenied == false {
								badgeToDeny.DeniedUserIDs = append(badgeToDeny.DeniedUserIDs, targetUser.ID)
								err = helpers.MDbUpdate(models.ProfileBadgesTable, badgeToDeny.ID, badgeToDeny)
								helpers.Relax(err)

								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(badgeToDeny.ID),
									models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
									models.EventlogTypeRobyulBadgeDeny, "",
									[]models.ElasticEventlogChange{
										{
											Key:      "badge_denieduserids",
											OldValue: strings.Join(deniedIDsBefore, ","),
											NewValue: strings.Join(badgeToDeny.DeniedUserIDs, ","),
											Type:     models.EventlogTargetTypeUser,
										},
									},
									[]models.ElasticEventlogOption{
										{
											Key:   "badge_category",
											Value: badgeToDeny.Category,
										},
										{
											Key:   "badge_name",
											Value: badgeToDeny.Name,
										},
										{
											Key:   "badge_url",
											Value: getBadgeUrl(badgeToDeny),
										},
										{
											Key:   "badge_bordercolor",
											Value: badgeToDeny.BorderColor,
										},
										{
											Key:   "badge_levelrequirement",
											Value: strconv.Itoa(badgeToDeny.LevelRequirement),
										},
										{
											Key:   "badge_rolerequirement",
											Value: badgeToDeny.RoleRequirement,
											Type:  models.EventlogTargetTypeRole,
										},
										{
											Key:   "badge_guildid",
											Value: badgeToDeny.GuildID,
											Type:  models.EventlogTargetTypeGuild,
										},
										{
											Key:   "badge_denieduserids_added",
											Value: targetUser.ID,
											Type:  models.EventlogTargetTypeUser,
										},
									}, false)
								helpers.RelaxLog(err)

								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.deny-badge-success-denied",
									targetUser.Username, badgeToDeny.Name, badgeToDeny.Category))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							} else {
								deniedUserIDsWithout := make([]string, 0)
								for _, userDeniedID := range badgeToDeny.DeniedUserIDs {
									if userDeniedID != targetUser.ID {
										deniedUserIDsWithout = append(deniedUserIDsWithout, userDeniedID)
									}
								}
								badgeToDeny.DeniedUserIDs = deniedUserIDsWithout
								err = helpers.MDbUpdate(models.ProfileBadgesTable, badgeToDeny.ID, badgeToDeny)
								helpers.Relax(err)

								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(badgeToDeny.ID),
									models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
									models.EventlogTypeRobyulBadgeDeny, "",
									[]models.ElasticEventlogChange{
										{
											Key:      "badge_denieduserids",
											OldValue: strings.Join(deniedIDsBefore, ","),
											NewValue: strings.Join(badgeToDeny.DeniedUserIDs, ","),
											Type:     models.EventlogTargetTypeUser,
										},
									},
									[]models.ElasticEventlogOption{
										{
											Key:   "badge_category",
											Value: badgeToDeny.Category,
										},
										{
											Key:   "badge_name",
											Value: badgeToDeny.Name,
										},
										{
											Key:   "badge_url",
											Value: getBadgeUrl(badgeToDeny),
										},
										{
											Key:   "badge_bordercolor",
											Value: badgeToDeny.BorderColor,
										},
										{
											Key:   "badge_levelrequirement",
											Value: strconv.Itoa(badgeToDeny.LevelRequirement),
										},
										{
											Key:   "badge_rolerequirement",
											Value: badgeToDeny.RoleRequirement,
											Type:  models.EventlogTargetTypeRole,
										},
										{
											Key:   "badge_guildid",
											Value: badgeToDeny.GuildID,
											Type:  models.EventlogTargetTypeGuild,
										},
										{
											Key:   "badge_denieduserids_removed",
											Value: targetUser.ID,
											Type:  models.EventlogTargetTypeUser,
										},
									}, false)
								helpers.RelaxLog(err)

								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.deny-badge-success-not-denied",
									targetUser.Username, badgeToDeny.Name, badgeToDeny.Category))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}
						})
						return
					case "move": // [p]profile badge move <category name> <badge name> <#>
						session.ChannelTyping(msg.ChannelID)
						if len(args) < 5 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}
						categoryName := args[2]
						badgeName := args[3]
						newSpot, err := strconv.Atoi(args[4])
						if err != nil {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						userData, err := m.GetUserUserdata(msg.Author)
						helpers.Relax(err)

						var idToMove string
						for _, badgeID := range userData.ActiveBadgeIDs {
							badge := getBadgeByID(badgeID)
							if badge.Category == categoryName && badge.Name == badgeName {
								idToMove = badge.GetID()
							}
						}

						if idToMove == "" {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-not-found"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						newBadgeList := make([]string, 0)
						badgeAdded := false
						for _, badgeID := range userData.ActiveBadgeIDs {
							if len(newBadgeList)+1 == newSpot {
								newBadgeList = append(newBadgeList, idToMove)
								badgeAdded = true
							}
							if badgeID != idToMove {
								newBadgeList = append(newBadgeList, badgeID)
							}
						}
						if badgeAdded == false && len(newBadgeList) < BadgeLimt {
							newBadgeList = append(newBadgeList, idToMove)
						}
						userData.ActiveBadgeIDs = newBadgeList
						err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
						helpers.Relax(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.move-badge-success"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

						return
					}
				}
				session.ChannelTyping(msg.ChannelID)

				availableBadges := getBadgesAvailable(msg.Author, channel.GuildID)

				if len(availableBadges) <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-none"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}

				userData, err := m.GetUserUserdata(msg.Author)
				helpers.Relax(err)
				newActiveBadgeIDs := make([]string, 0)
				for _, activeBadgeID := range userData.ActiveBadgeIDs {
					for _, availableBadge := range availableBadges {
						if availableBadge.GetID() == activeBadgeID {
							newActiveBadgeIDs = append(newActiveBadgeIDs, activeBadgeID)
						}
					}
				}
				userData.ActiveBadgeIDs = newActiveBadgeIDs

				shownBadges := make([]models.ProfileBadgeEntry, 0)
				for _, badge := range availableBadges {
					if badge.GuildID == "global" || badge.GuildID == channel.GuildID {
						shownBadges = append(shownBadges, badge)
					}
				}

				inCategory := ""
				stoppedLoop := false
				lastBotMessageID := make([]string, 0)
				closeHandler := session.AddHandler(func(session *discordgo.Session, loopMessage *discordgo.MessageCreate) {
					if stoppedLoop == false && loopMessage.Author.ID == msg.Author.ID && loopMessage.ChannelID == msg.ChannelID {
						cleanedContent := strings.Replace(loopMessage.Content, "_profile ", "", 1)
						cleanedContent = strings.Replace(cleanedContent, "category ", "", 1)
						cleanedContent = strings.Replace(cleanedContent, "badge ", "", 1)
						loopArgs := strings.Fields(strings.ToLower(cleanedContent))
					BadgePickLoop:
						for {
							if len(loopArgs) > 0 {
								switch loopArgs[0] {
								case "stop", "exit":
									err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
									helpers.Relax(err)
									m.DeleteMessages(msg.ChannelID, lastBotMessageID)
									_, err = helpers.SendMessage(msg.ChannelID,
										fmt.Sprintf("**@%s** I saved your badges. Check out your new shiny profile with `_profile` :sparkles: \n", msg.Author.Username))
									helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
									stoppedLoop = true
									newActiveBadgePickerUserIDs := make(map[string]string, 0)
									for activeBadgePickerUserID, activeBadgePickerChannelID := range activeBadgePickerUserIDs {
										if activeBadgePickerUserID != msg.Author.ID {
											newActiveBadgePickerUserIDs[activeBadgePickerUserID] = activeBadgePickerChannelID
										}
									}
									activeBadgePickerUserIDs = newActiveBadgePickerUserIDs
									return
								case "reset":
									userData.ActiveBadgeIDs = make([]string, 0)
									loopArgs = []string{"categories"}
									continue BadgePickLoop
									return
								case "categories":
									inCategory = ""
									m.DeleteMessages(msg.ChannelID, lastBotMessageID)
									lastBotMessageID = m.BadgePickerPrintCategories(msg.Author, msg.ChannelID, shownBadges, userData.ActiveBadgeIDs, availableBadges)
									return
								default:
									if inCategory == "" {
										for _, badge := range shownBadges {
											if badge.Category == strings.ToLower(loopArgs[0]) {
												m.DeleteMessages(msg.ChannelID, lastBotMessageID)
												inCategory = strings.ToLower(loopArgs[0])
												lastBotMessageID = m.BadgePickerPrintBadges(msg.Author, msg.ChannelID, shownBadges, userData.ActiveBadgeIDs, inCategory, availableBadges)
												return
											}
										}
										m.DeleteMessages(msg.ChannelID, lastBotMessageID)
										messages, err := helpers.SendMessage(msg.ChannelID,
											fmt.Sprintf("**@%s** I wasn't able to find a category with that name.\n%s", msg.Author.Username, m.BadgePickerHelpText()))
										helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
										lastBotMessageID = []string{}
										for _, message := range messages {
											lastBotMessageID = append(lastBotMessageID, message.ID)
										}
										return
									} else {
										for _, badge := range shownBadges {
											if badge.Category == inCategory && badge.Name == strings.ToLower(loopArgs[0]) {
												for _, activeBadgeID := range userData.ActiveBadgeIDs {
													if activeBadgeID == badge.GetID() {
														newActiveBadges := make([]string, 0)
														for _, newActiveBadgeID := range userData.ActiveBadgeIDs {
															if newActiveBadgeID != badge.GetID() {
																newActiveBadges = append(newActiveBadges, newActiveBadgeID)
															}
														}
														loopArgs = []string{"categories"}
														userData.ActiveBadgeIDs = newActiveBadges
														continue BadgePickLoop
													}
												}
												if len(userData.ActiveBadgeIDs) >= BadgeLimt {
													m.DeleteMessages(msg.ChannelID, lastBotMessageID)
													messages, err := helpers.SendMessage(msg.ChannelID,
														fmt.Sprintf("**@%s** You are already got enough emotes.\n%s", msg.Author.Username, m.BadgePickerHelpText()))
													helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
													lastBotMessageID = []string{}
													for _, message := range messages {
														lastBotMessageID = append(lastBotMessageID, message.ID)
													}
													return
												}

												loopArgs = []string{"categories"}
												userData.ActiveBadgeIDs = append(userData.ActiveBadgeIDs, badge.GetID())
												if len(userData.ActiveBadgeIDs) >= BadgeLimt {
													err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
													helpers.Relax(err)
													m.DeleteMessages(msg.ChannelID, lastBotMessageID)
													_, err = helpers.SendMessage(msg.ChannelID,
														fmt.Sprintf("**@%s** I saved your badges. Check out your new shiny profile with `_profile` :sparkles: \n",
															msg.Author.Username))
													helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
													stoppedLoop = true
													newActiveBadgePickerUserIDs := make(map[string]string, 0)
													for activeBadgePickerUserID, activeBadgePickerChannelID := range activeBadgePickerUserIDs {
														if activeBadgePickerUserID != msg.Author.ID {
															newActiveBadgePickerUserIDs[activeBadgePickerUserID] = activeBadgePickerChannelID
														}
													}
													activeBadgePickerUserIDs = newActiveBadgePickerUserIDs
													return
												}
												continue BadgePickLoop
											}
										}
										m.DeleteMessages(msg.ChannelID, lastBotMessageID)
										messages, err := helpers.SendMessage(msg.ChannelID,
											fmt.Sprintf("**@%s** I wasn't able to find a badge with that name.\n%s", msg.Author.Username, m.BadgePickerHelpText()))
										helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
										lastBotMessageID = []string{}
										for _, message := range messages {
											lastBotMessageID = append(lastBotMessageID, message.ID)
										}
										return
									}
								}
							}
							return
						}
					}
				})
				activeBadgePickerUserIDs[msg.Author.ID] = msg.ChannelID
				lastBotMessageID = m.BadgePickerPrintCategories(msg.Author, msg.ChannelID, shownBadges, userData.ActiveBadgeIDs, availableBadges)
				time.Sleep(5 * time.Minute)
				closeHandler()
				if stoppedLoop == false {
					err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
					helpers.Relax(err)
					newActiveBadgePickerUserIDs := make(map[string]string, 0)
					for activeBadgePickerUserID, activeBadgePickerChannelID := range activeBadgePickerUserIDs {
						if activeBadgePickerUserID != msg.Author.ID {
							newActiveBadgePickerUserIDs[activeBadgePickerUserID] = activeBadgePickerChannelID
						}
					}
					activeBadgePickerUserIDs = newActiveBadgePickerUserIDs

					m.DeleteMessages(msg.ChannelID, lastBotMessageID)
					_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("**@%s** I stopped the badge picking and saved your badges because of the time limit.\nUse `_profile badge` if you want to pick more badges.",
						msg.Author.Username))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				}
				return
			case "color", "colour":
				session.ChannelTyping(msg.ChannelID)
				if len(args) < 2 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}

				userUserdata, err := m.GetUserUserdata(msg.Author)
				helpers.Relax(err)

				switch args[1] {
				case "background", "box":
					if len(args) >= 3 {
						userUserdata.BackgroundColor = strings.Replace(args[2], "#", "", -1)
					} else {
						userUserdata.BackgroundColor = ""
					}
				case "accent":
					if len(args) >= 3 {
						userUserdata.AccentColor = strings.Replace(args[2], "#", "", -1)
					} else {
						userUserdata.AccentColor = ""
					}
				case "text":
					if len(args) >= 3 {
						userUserdata.TextColor = strings.Replace(args[2], "#", "", -1)
					} else {
						userUserdata.TextColor = ""
					}
				default:
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-color-set-success"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "opacity":
				session.ChannelTyping(msg.ChannelID)
				if len(args) < 2 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}

				userUserdata, err := m.GetUserUserdata(msg.Author)
				helpers.Relax(err)

				opacityText := "0.5"
				switch args[1] {
				case "badge", "badges":
					opacityText = "1.0"
				}
				if len(args) >= 3 {
					opacity, err := strconv.ParseFloat(args[2], 64)
					if err != nil {
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
					opacityText = fmt.Sprintf("%.1f", opacity)
				}

				switch args[1] {
				case "background", "box":
					userUserdata.BackgroundOpacity = opacityText
				case "details", "detail":
					userUserdata.DetailOpacity = opacityText
				case "exp", "EXP", "expbar":
					userUserdata.EXPOpacity = opacityText
				case "badge", "badges":
					userUserdata.BadgeOpacity = opacityText
				default:
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}

				err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-opacity-set-success"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "timezone":
				userUserdata, err := m.GetUserUserdata(msg.Author)
				helpers.Relax(err)

				newTimezoneString := ""
				timeInTimezone := ""
				if len(args) < 2 {
					if userUserdata.Timezone == "" {
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-timezone-list"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
				} else {
					loc, err := time.LoadLocation(args[1])
					if err != nil {
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-timezone-set-error")+"\n"+helpers.GetText("plugins.levels.profile-timezone-list"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
					newTimezoneString = loc.String()
					timeInTimezone = time.Now().In(loc).Format(TimeAtUserFormat)
				}
				userUserdata.Timezone = newTimezoneString
				err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
				helpers.Relax(err)

				if timeInTimezone != "" {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.profile-timezone-set-success",
						newTimezoneString, timeInTimezone))
				} else {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.profile-timezone-reset-success"))
				}
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "birthday":
				var err error

				newBirthday := ""
				if len(args) >= 2 {
					_, err = time.Parse(TimeBirthdayFormat, args[1])
					if err != nil {
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-birthday-set-error-format"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
					newBirthday = args[1]
				}

				userUserdata, err := m.GetUserUserdata(msg.Author)
				helpers.Relax(err)
				userUserdata.Birthday = newBirthday
				err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-birthday-set-success"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}

			targetUser, err = helpers.GetUserFromMention(args[0])
			if targetUser == nil || targetUser.ID == "" {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}
		}

		targetMember, err := helpers.GetGuildMember(channel.GuildID, targetUser.ID)
		if errD, ok := err.(*discordgo.RESTError); ok {
			if errD.Message.Code == 10007 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			} else {
				helpers.Relax(err)
			}
		} else {
			helpers.Relax(err)
		}

		gifP := false
		if command == "gif-profile" {
			gifP = true
		}

		jpgBytes, ext, err := m.GetProfile(targetMember, guild, gifP)
		if err != nil {
			cache.GetLogger().WithField("module", "levels").Error(fmt.Sprintf("Profile generation failed: %#v", err))
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-error-exit1"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}

		_, err = helpers.SendComplex(
			msg.ChannelID, &discordgo.MessageSend{
				Content: fmt.Sprintf("<@%s> Profile for %s", msg.Author.ID, targetUser.Username),
				Files: []*discordgo.File{
					{
						Name:   fmt.Sprintf("%s-Robyul.%s", targetUser.ID, ext),
						Reader: bytes.NewReader(jpgBytes),
					},
				},
			})
		if err != nil {
			if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == 20009 {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-error-sending"))
				return
			}
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		}

		return
	case "level", "levels": // [p]level <user> or [p]level top
		session.ChannelTyping(msg.ChannelID)
		targetUser, err := helpers.GetUser(msg.Author.ID)
		helpers.Relax(err)
		args := strings.Fields(content)

		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)
		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)

		if len(args) >= 1 && args[0] != "" {
			switch args[0] {
			case "leaderboard", "top":
				// [p]level top
				// TODO: use cached top list
				var levelsServersUsers []models.LevelsServerusersEntry
				err := helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"guildid": channel.GuildID}).Sort("-exp").Limit(10)).All(&levelsServersUsers)
				helpers.Relax(err)

				if levelsServersUsers == nil || len(levelsServersUsers) <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.top-server-no-stats"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				} else if err != nil {
					helpers.Relax(err)
				}

				rankingUrl := helpers.GetConfig().Path("website.ranking_base_url").Data().(string) + "/" + channel.GuildID
				topLevelEmbed := &discordgo.MessageEmbed{
					Color:       0x0FADED,
					Title:       helpers.GetTextF("plugins.levels.top-server-embed-title", guild.Name),
					Description: "View the leaderboard for this server [here](" + rankingUrl + ").",
					Fields:      []*discordgo.MessageEmbedField{},
					URL:         rankingUrl,
				}

				displayRanking := 1
				offset := 0
				for i := 0; displayRanking <= 10; i++ {
					if len(levelsServersUsers) <= i-offset {
						offset += i
						err = helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"guildid": channel.GuildID}).Skip(offset).Sort("-exp").Limit(5)).All(&levelsServersUsers)
						helpers.Relax(err)
						if levelsServersUsers == nil {
							break
						}
					}
					if len(levelsServersUsers) <= i-offset {
						break
					}

					currentMember, err := helpers.GetGuildMember(channel.GuildID, levelsServersUsers[i-offset].UserID)
					if err != nil {
						cache.GetLogger().WithField("module", "levels").Error(fmt.Sprintf("error fetching member data for user #%s: %s", levelsServersUsers[i-offset].UserID, err.Error()))
						continue
					}
					fullUsername := currentMember.User.Username
					if currentMember.Nick != "" {
						fullUsername += " ~ " + currentMember.Nick
					}

					topLevelEmbed.Fields = append(topLevelEmbed.Fields, &discordgo.MessageEmbedField{
						Name:   fmt.Sprintf("%d. %s", displayRanking, fullUsername),
						Value:  fmt.Sprintf("Level: %d", getLevelFromExp(levelsServersUsers[i-offset].Exp)),
						Inline: false,
					})
					displayRanking++
				}

				var thislevelUser models.LevelsServerusersEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"guildid": channel.GuildID, "userid": targetUser.ID}),
					&thislevelUser,
				)
				if err == nil && thislevelUser.ID.Valid() {
					serverRank := "N/A"
					rank := 1
					for _, serverCache := range topCache {
						if serverCache.GuildID == channel.GuildID {
							for _, pair := range serverCache.Levels {
								if _, err = helpers.GetGuildMemberWithoutApi(serverCache.GuildID, pair.Key); err != nil {
									continue
								}
								if pair.Key == targetUser.ID {
									// substract skipped members to ignore users that left in the ranking
									serverRank = strconv.Itoa(rank)
									break
								}
								rank++
							}
						}
					}

					topLevelEmbed.Fields = append(topLevelEmbed.Fields, &discordgo.MessageEmbedField{
						Name:   "Your Rank: " + serverRank,
						Value:  fmt.Sprintf("Level: %d", getLevelFromExp(thislevelUser.Exp)),
						Inline: false,
					})

				}

				if guild.Icon != "" {
					topLevelEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: discordgo.EndpointGuildIcon(guild.ID, guild.Icon)}
				}

				_, err = helpers.SendEmbed(msg.ChannelID, topLevelEmbed)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "global-leaderboard", "global-top", "globaltop":
				var rankedTotalExpMap PairList
				for _, serverCache := range topCache {
					if serverCache.GuildID == "global" {
						rankedTotalExpMap = serverCache.Levels
					}
				}

				if len(rankedTotalExpMap) <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.no-stats-available-yet"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}

				rankingUrl := helpers.GetConfig().Path("website.ranking_base_url").Data().(string)
				globalTopLevelEmbed := &discordgo.MessageEmbed{
					Color:       0x0FADED,
					Title:       helpers.GetText("plugins.levels.global-top-server-embed-title"),
					Description: "View the global leaderboard [here](" + rankingUrl + ").",
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.levels.embed-footer",
						len(session.State.Guilds),
					)},
					Fields: []*discordgo.MessageEmbedField{},
					URL:    rankingUrl,
				}

				i := 0
				for _, userRanked := range rankedTotalExpMap {
					currentUser, err := helpers.GetUser(userRanked.Key)
					if err != nil {
						cache.GetLogger().WithField("module", "levels").Error(fmt.Sprintf("error fetching user data for user #%s: %s", userRanked.Key, err.Error()))
						continue
					}
					fullUsername := currentUser.Username
					globalTopLevelEmbed.Fields = append(globalTopLevelEmbed.Fields, &discordgo.MessageEmbedField{
						Name:   fmt.Sprintf("%d. %s", i+1, fullUsername),
						Value:  fmt.Sprintf("Global Level: %d", getLevelFromExp(userRanked.Value)),
						Inline: false,
					})
					i++
					if i >= 10 {
						break
					}
				}

				var thislevelServersUser []models.LevelsServerusersEntry
				err = helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"userid": targetUser.ID})).All(&thislevelServersUser)
				helpers.Relax(err)

				if thislevelServersUser != nil {
					var totalExp int64
					for _, levelServerUser := range thislevelServersUser {
						totalExp += levelServerUser.Exp
					}

					globalRank := "N/A"
					for _, serverCache := range topCache {
						if serverCache.GuildID == "global" {
							for i, pair := range serverCache.Levels {
								if pair.Key == targetUser.ID {
									globalRank = strconv.Itoa(i + 1)
								}
							}
						}
					}

					globalTopLevelEmbed.Fields = append(globalTopLevelEmbed.Fields, &discordgo.MessageEmbedField{
						Name:   "Your Rank: " + globalRank,
						Value:  fmt.Sprintf("Global Level: %d", getLevelFromExp(totalExp)),
						Inline: false,
					})
				}

				_, err = helpers.SendEmbed(msg.ChannelID, globalTopLevelEmbed)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "reset":
				if len(args) >= 2 {
					switch args[1] {
					case "user": // [p]levels reset user <user>
						if len(args) < 3 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						helpers.RequireAdmin(msg, func() {
							targetUser, err = helpers.GetUserFromMention(args[2])
							if targetUser == nil || targetUser.ID == "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							levelsServerUser, err := m.getLevelsServerUserOrCreateNew(channel.GuildID, targetUser.ID)
							helpers.Relax(err)

							expBefore := levelsServerUser.Exp

							levelsServerUser.Exp = 0
							err = helpers.MDbUpdate(models.LevelsServerusersTable, levelsServerUser.ID, levelsServerUser)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
								models.EventlogTargetTypeUser, msg.Author.ID,
								models.EventlogTypeRobyulLevelsReset, "",
								[]models.ElasticEventlogChange{
									{
										Key:      "levels_exp",
										OldValue: strconv.FormatInt(expBefore, 10),
										NewValue: strconv.FormatInt(levelsServerUser.Exp, 10),
									},
								},
								nil, false)
							helpers.RelaxLog(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-resetted"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						})
						return
					}
				}
				return
			case "ignore":
				if len(args) >= 2 {
					switch args[1] {
					case "list": // [p]levels ignore list
						helpers.RequireMod(msg, func() {
							settings := helpers.GuildSettingsGetCached(channel.GuildID)

							ignoredMessage := "**Ignored Users:**"
							if len(settings.LevelsIgnoredUserIDs) > 0 {
								for i, ignoredUserID := range settings.LevelsIgnoredUserIDs {
									ignoredUser, err := helpers.GetGuildMemberWithoutApi(channel.GuildID, ignoredUserID)
									if err != nil {
										ignoredMessage += " N/A"
									} else {
										ignoredMessage += " " + ignoredUser.User.Username + "#" + ignoredUser.User.Discriminator
									}
									if i+1 < len(settings.LevelsIgnoredUserIDs) {
										ignoredMessage += ","
									}
								}
							} else {
								ignoredMessage += " None"
							}

							ignoredMessage += "\n**Ignored Channels:**"
							if len(settings.LevelsIgnoredChannelIDs) > 0 {
								for i, ignoredChannelID := range settings.LevelsIgnoredChannelIDs {
									ignoredMessage += " <#" + ignoredChannelID + ">"
									if i+1 < len(settings.LevelsIgnoredChannelIDs) {
										ignoredMessage += ","
									}
								}
							} else {
								ignoredMessage += " None"
							}

							for _, page := range helpers.Pagify(ignoredMessage, " ") {
								_, err = helpers.SendMessage(msg.ChannelID, page)
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							}
							return
						})
						return
					case "user": // [p]levels ignore user <user>
						if len(args) < 3 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						helpers.RequireAdmin(msg, func() {
							targetUser, err = helpers.GetUserFromMention(args[2])
							if targetUser == nil || targetUser.ID == "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							settings := helpers.GuildSettingsGetCached(channel.GuildID)

							ignoredUserIDsBefore := settings.LevelsIgnoredUserIDs

							for i, ignoredUserID := range settings.LevelsIgnoredUserIDs {
								if ignoredUserID == targetUser.ID {
									settings.LevelsIgnoredUserIDs = append(settings.LevelsIgnoredUserIDs[:i], settings.LevelsIgnoredUserIDs[i+1:]...)
									err = helpers.GuildSettingsSet(channel.GuildID, settings)
									helpers.Relax(err)

									_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
										models.EventlogTargetTypeUser, msg.Author.ID,
										models.EventlogTypeRobyulLevelsIgnoreUser, "",
										[]models.ElasticEventlogChange{
											{
												Key:      "levels_ignoreduserids",
												OldValue: strings.Join(ignoredUserIDsBefore, ","),
												NewValue: strings.Join(settings.LevelsIgnoredUserIDs, ","),
												Type:     models.EventlogTargetTypeUser,
											},
										},
										[]models.ElasticEventlogOption{
											{
												Key:   "levels_ignoreduserids_removed",
												Value: targetUser.ID,
												Type:  models.EventlogTargetTypeUser,
											},
										}, false)
									helpers.RelaxLog(err)

									_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.ignore-user-removed"))
									helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
									return
								}
							}

							settings.LevelsIgnoredUserIDs = append(settings.LevelsIgnoredUserIDs, targetUser.ID)
							err = helpers.GuildSettingsSet(channel.GuildID, settings)
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
								models.EventlogTargetTypeUser, msg.Author.ID,
								models.EventlogTypeRobyulLevelsIgnoreUser, "",
								[]models.ElasticEventlogChange{
									{
										Key:      "levels_ignoreduserids",
										OldValue: strings.Join(ignoredUserIDsBefore, ","),
										NewValue: strings.Join(settings.LevelsIgnoredUserIDs, ","),
										Type:     models.EventlogTargetTypeUser,
									},
								},
								[]models.ElasticEventlogOption{
									{
										Key:   "levels_ignoreduserids_added",
										Value: targetUser.ID,
										Type:  models.EventlogTargetTypeUser,
									},
								}, false)
							helpers.RelaxLog(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.ignore-user-added", helpers.GetPrefixForServer(channel.GuildID)))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						})
						return
					case "channel": // [p]levels ignore channel <channel>
						if len(args) < 3 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						helpers.RequireAdmin(msg, func() {
							targetChannel, err := helpers.GetChannelFromMention(msg, args[2])
							helpers.Relax(err)
							if targetChannel == nil || targetChannel.ID == "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							settings := helpers.GuildSettingsGetCached(channel.GuildID)

							ignoredChannelIDsBefore := settings.LevelsIgnoredChannelIDs

							for i, ignoredChannelID := range settings.LevelsIgnoredChannelIDs {
								if ignoredChannelID == targetChannel.ID {
									settings.LevelsIgnoredChannelIDs = append(settings.LevelsIgnoredChannelIDs[:i], settings.LevelsIgnoredChannelIDs[i+1:]...)
									err = helpers.GuildSettingsSet(channel.GuildID, settings)
									helpers.Relax(err)

									_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetChannel.ID,
										models.EventlogTargetTypeChannel, msg.Author.ID,
										models.EventlogTypeRobyulLevelsIgnoreChannel, "",
										[]models.ElasticEventlogChange{
											{
												Key:      "levels_ignoredchannelids",
												OldValue: strings.Join(ignoredChannelIDsBefore, ","),
												NewValue: strings.Join(settings.LevelsIgnoredChannelIDs, ","),
												Type:     models.EventlogTargetTypeChannel,
											},
										},
										[]models.ElasticEventlogOption{
											{
												Key:   "levels_ignoredchannelids_removed",
												Value: targetChannel.ID,
												Type:  models.EventlogTargetTypeChannel,
											},
										}, false)
									helpers.RelaxLog(err)

									_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.ignore-channel-removed"))
									helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
									return
								}
							}

							settings.LevelsIgnoredChannelIDs = append(settings.LevelsIgnoredChannelIDs, targetChannel.ID)
							err = helpers.GuildSettingsSet(channel.GuildID, settings)
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetChannel.ID,
								models.EventlogTargetTypeChannel, msg.Author.ID,
								models.EventlogTypeRobyulLevelsIgnoreChannel, "",
								[]models.ElasticEventlogChange{
									{
										Key:      "levels_ignoredchannelids",
										OldValue: strings.Join(ignoredChannelIDsBefore, ","),
										NewValue: strings.Join(settings.LevelsIgnoredChannelIDs, ","),
										Type:     models.EventlogTargetTypeChannel,
									},
								},
								[]models.ElasticEventlogOption{
									{
										Key:   "levels_ignoredchannelids_added",
										Value: targetChannel.ID,
										Type:  models.EventlogTargetTypeChannel,
									},
								}, false)
							helpers.RelaxLog(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.ignore-channel-added"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						})
						return
					}
				}
				return
				// [p]level process-history
			case "process-history":
				helpers.RequireBotAdmin(msg, func() {
					dmChannel, err := session.UserChannelCreate(msg.Author.ID)
					helpers.Relax(err)
					session.ChannelTyping(msg.ChannelID)
					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)
					guild, err := helpers.GetGuild(channel.GuildID)
					helpers.Relax(err)
					_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> Check your DMs.", msg.Author.ID))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					// pause new message processing for that guild
					temporaryIgnoredGuilds = append(temporaryIgnoredGuilds, channel.GuildID)
					_, err = helpers.SendMessage(dmChannel.ID, fmt.Sprintf("Temporary disabled EXP Processing for `%s` while processing the Message History.", guild.Name))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					// reset accounts on this server
					var levelsServersUsers []models.LevelsServerusersEntry
					err = helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"guildid": channel.GuildID})).All(&levelsServersUsers)
					helpers.Relax(err)
					if levelsServersUsers != nil {
						for _, levelsServerUser := range levelsServersUsers {
							levelsServerUser.Exp = 0
							err = helpers.MDbUpdate(models.LevelsServerusersTable, levelsServerUser.ID, levelsServerUser)
							helpers.Relax(err)
						}
					}
					_, err = helpers.SendMessage(dmChannel.ID, fmt.Sprintf("Resetted the EXP for every User on `%s`.", guild.Name))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					// process history
					//var wg sync.WaitGroup
					//wg.Add(len(guild.Channels))
					for _, guildChannel := range guild.Channels {
						guildChannelCurrent := guildChannel
						//go func() {
						prefix := helpers.GetPrefixForServer(guildChannelCurrent.GuildID)
						expForUsers := make(map[string]int64)
						//defer wg.Done()
						if guildChannelCurrent.Type == discordgo.ChannelTypeGuildVoice {
							continue
						}

						cache.GetLogger().WithField("module", "levels").Info(fmt.Sprintf("Started processing of Channel #%s (#%s) on Guild %s (#%s)",
							guildChannelCurrent.Name, guildChannelCurrent.ID, guild.Name, guild.ID))
						// (asynchronous)
						_, err = helpers.SendMessage(dmChannel.ID, fmt.Sprintf("Started processing Messages for Channel <#%s>.", guildChannelCurrent.ID))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						lastBefore := ""
						for {
							messages, err := session.ChannelMessages(guildChannelCurrent.ID, 100, lastBefore, "", "")
							if err != nil {
								cache.GetLogger().WithField("module", "levels").Error(err.Error())
								break
							}
							cache.GetLogger().WithField("module", "levels").Info(fmt.Sprintf("Processing %d messages for Channel #%s (#%s) from before \"%s\" on Guild %s (#%s)",
								len(messages), guildChannelCurrent.Name, guildChannelCurrent.ID, lastBefore, guild.Name, guild.ID))
							if len(messages) <= 0 {
								break
							}
							for _, message := range messages {
								// ignore bot messages
								if message.Author.Bot == true {
									continue
								}
								// ignore commands
								if prefix != "" {
									if strings.HasPrefix(message.Content, prefix) {
										continue
									}
								}
								if _, ok := expForUsers[message.Author.ID]; ok {
									expForUsers[message.Author.ID] += 5
								} else {
									expForUsers[message.Author.ID] = 5
								}

							}
							lastBefore = messages[len(messages)-1].ID
						}

						for userId, expForuser := range expForUsers {
							levelsServerUser, err := m.getLevelsServerUserOrCreateNew(guildChannelCurrent.GuildID, userId)
							helpers.Relax(err)
							levelsServerUser.Exp += expForuser
							err = helpers.MDbUpdate(models.LevelsServerusersTable, levelsServerUser.ID, levelsServerUser)
							helpers.Relax(err)
						}

						cache.GetLogger().WithField("module", "levels").Info(fmt.Sprintf("Completed processing of Channel #%s (#%s) on Guild %s (#%s)",
							guildChannelCurrent.Name, guildChannelCurrent.ID, guild.Name, guild.ID))
						_, err = helpers.SendMessage(dmChannel.ID, fmt.Sprintf("Completed processing Messages for Channel <#%s>.", guildChannelCurrent.ID))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						//}()
					}
					//fmt.Println("Waiting for all channels")
					//wg.Wait()
					// enable new message processing again
					var newTemporaryIgnoredGuilds []string
					for _, temporaryIgnoredGuild := range temporaryIgnoredGuilds {
						if temporaryIgnoredGuild != channel.GuildID {
							newTemporaryIgnoredGuilds = append(newTemporaryIgnoredGuilds, temporaryIgnoredGuild)
						}
					}
					temporaryIgnoredGuilds = newTemporaryIgnoredGuilds
					_, err = helpers.SendMessage(dmChannel.ID, fmt.Sprintf("Enabled EXP Processing for `%s` again.", guild.Name))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
						models.EventlogTargetTypeGuild, msg.Author.ID,
						models.EventlogTypeRobyulLevelsProcessedHistory, "",
						nil,
						nil, false)
					helpers.RelaxLog(err)

					_, err = helpers.SendMessage(dmChannel.ID, "Done!")
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				})
				return
			case "role", "roles":
				if len(args) < 2 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}

				switch args[1] {
				case "add":
					helpers.RequireMod(msg, func() {
						// [p]levels role add <role name or id> <start level> [<last level>]
						if len(args) < 4 {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}
						if _, err = strconv.Atoi(args[len(args)-1]); err != nil {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						serverRoles, err := session.GuildRoles(channel.GuildID)
						if err != nil {
							if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
								_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.permissions.required", "Manage Roles"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}
						}
						helpers.Relax(err)

						var targetRole *discordgo.Role

						roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))

						var startLevel int
						lastLevel := -1
						if _, err = strconv.Atoi(args[len(args)-2]); len(args) > 4 && err == nil {
							startLevel, err = strconv.Atoi(args[len(args)-2])
							helpers.Relax(err)
							lastLevel, err = strconv.Atoi(args[len(args)-1])
							helpers.Relax(err)

							roleNameToMatch = strings.TrimSpace(strings.Replace(roleNameToMatch, " "+strings.Join(args[len(args)-2:], " "), "", 1))
						} else {
							startLevel, err = strconv.Atoi(args[len(args)-1])
							helpers.Relax(err)

							roleNameToMatch = strings.TrimSpace(strings.Replace(roleNameToMatch, " "+strings.Join(args[len(args)-1:], " "), "", 1))
						}

						for _, role := range serverRoles {
							if strings.ToLower(role.Name) == strings.ToLower(roleNameToMatch) || role.ID == roleNameToMatch {
								targetRole = role
							}
						}

						if targetRole == nil || targetRole.ID == "" || startLevel < 0 || (lastLevel < 0 && lastLevel != -1) || (lastLevel != -1 && startLevel > lastLevel) {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						_, err = m.createLevelsRoleEntry(channel.GuildID, targetRole.ID, startLevel, lastLevel)
						helpers.Relax(err)

						options := []models.ElasticEventlogOption{
							{
								Key:   "role_startlevel",
								Value: strconv.Itoa(startLevel),
							},
						}

						if lastLevel >= 0 {
							options = append(options, models.ElasticEventlogOption{
								Key:   "role_lastlevel",
								Value: strconv.Itoa(lastLevel),
							})
						}

						_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetRole.ID,
							models.EventlogTargetTypeRole, msg.Author.ID,
							models.EventlogTypeRobyulLevelsRoleAdd, "",
							nil,
							options, false)
						helpers.RelaxLog(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.levels-role-add-success", targetRole.Name))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					})
					return
				case "apply":
					// [p]levels role apply
					helpers.RequireMod(msg, func() {
						if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetText("plugins.levels.levels-role-apply-confirm"), "✅", "🚫") {
							errors := make([]error, 0)
							var success int

							guild, err := helpers.GetGuild(channel.GuildID)
							helpers.Relax(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.levels-role-apply-start"))

							for _, member := range guild.Members {
								if member.User.Bot == true {
									continue
								}

								errRole := applyLevelsRoles(guild.ID, member.User.ID, getLevelForUser(member.User.ID, guild.ID))
								if errRole == nil {
									success++
								} else {
									errors = append(errors, errRole)
								}
							}

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
								models.EventlogTargetTypeGuild, msg.Author.ID,
								models.EventlogTypeRobyulLevelsRoleApply, "",
								nil,
								[]models.ElasticEventlogOption{
									{
										Key:   "roles_added",
										Value: strconv.Itoa(success),
									},
									{
										Key:   "roles_errors",
										Value: strconv.Itoa(len(errors)),
									},
								}, false)
							helpers.RelaxLog(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.levels-role-apply-result", msg.Author.ID, success, len(errors)))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}
						return
					})
					return
				case "list":
					// [p]levels role list
					helpers.RequireMod(msg, func() {
						entries, err := m.getLevelsRoleEntriesBy("guild_id", channel.GuildID)
						if err != nil {
							if !strings.Contains(err.Error(), "no levels role entries") {
								helpers.Relax(err)
							}
						}

						if len(entries) <= 0 {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.levels-role-list-empty"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						}

						var message string
						for _, entry := range entries {
							role, err := session.State.Role(entry.GuildID, entry.RoleID)
							if err != nil {
								role = new(discordgo.Role)
								role.ID = "N/A"
								role.Name = "N/A"
							}

							lastLevelText := strconv.Itoa(entry.LastLevel)
							if lastLevelText == "-1" {
								lastLevelText = "∞"
							}

							message += fmt.Sprintf("`%s`: Role `%s` (`#%s`) from level %d to level %s\n",
								entry.ID, role.Name, role.ID, entry.StartLevel, lastLevelText)
						}
						message += fmt.Sprintf("_found %d role(s) in total_", len(entries))

						overwrites := m.getLevelsRolesGuildOverwrites(channel.GuildID)

						if len(overwrites) > 0 {
							message += "\n**Overwrites:**\n"

							for _, overwrite := range overwrites {
								overwriteRole, err := session.State.Role(channel.GuildID, overwrite.RoleID)
								if err != nil {
									continue
								}
								overwriteUser, err := helpers.GetUser(overwrite.UserID)
								if err != nil {
									overwriteUser = new(discordgo.User)
									overwriteUser.Username = "N/A"
									overwriteUser.ID = overwrite.UserID
								}

								message += fmt.Sprintf("%s user `%s` (`#%s`) role `%s` (`#%s`)\n",
									strings.Title(overwrite.Type), overwriteUser.Username, overwriteUser.ID, overwriteRole.Name, overwriteRole.ID)
							}
							message += fmt.Sprintf("_found %d overwrite(s) in total_", len(overwrites))
						}

						for _, page := range helpers.Pagify(message, "\n") {
							_, err = helpers.SendMessage(msg.ChannelID, page)
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						}
						return
					})
					return
				case "remove", "delete":
					// levels role remove <connection id>
					helpers.RequireMod(msg, func() {
						if len(args) < 3 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						entry, err := m.getLevelsRoleEntryBy("id", args[2])
						if err != nil {
							if !strings.Contains(err.Error(), "no levels role entry") {
								helpers.Relax(err)
							}
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						err = m.deleteLevelsRoleEntry(entry)
						helpers.Relax(err)

						role, err := session.State.Role(channel.GuildID, entry.RoleID)
						if err != nil {
							role = new(discordgo.Role)
							role.Name = "N/A"
						}

						options := []models.ElasticEventlogOption{
							{
								Key:   "role_startlevel",
								Value: strconv.Itoa(entry.StartLevel),
							},
						}

						if entry.LastLevel >= 0 {
							options = append(options, models.ElasticEventlogOption{
								Key:   "role_lastlevel",
								Value: strconv.Itoa(entry.LastLevel),
							})
						}

						_, err = helpers.EventlogLog(time.Now(), channel.GuildID, entry.RoleID,
							models.EventlogTargetTypeRole, msg.Author.ID,
							models.EventlogTypeRobyulLevelsRoleDelete, "",
							nil,
							options, false)
						helpers.RelaxLog(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.levels-role-delete-success",
							role.Name, entry.RoleID))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					})
					return
				case "grant":
					// TODO: apply roles on join, show overwrites in list
					helpers.RequireMod(msg, func() {
						if len(args) < 4 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						guild, err := helpers.GetGuild(channel.GuildID)
						helpers.Relax(err)

						targetUser, err := helpers.GetUserFromMention(args[2])
						if err != nil || targetUser == nil || targetUser.ID == "" {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						var targetRole *discordgo.Role

						roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:3], " "), "", 1))
						for _, guildRole := range guild.Roles {
							if strings.ToLower(guildRole.Name) == strings.ToLower(roleNameToMatch) || guildRole.ID == roleNameToMatch {
								targetRole = guildRole
							}
						}

						if targetRole == nil || targetRole.ID == "" {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						previousGrant, previousDeny, grant := m.getLevelsRolesUserRoleOverwrite(guild.ID, targetRole.ID, targetUser.ID)

						if previousDeny {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.roles-grant-error-denying"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						if previousGrant {
							err = m.deleteLevelsRolesOverwriteEntry(grant)

							err = applyLevelsRoles(guild.ID, targetUser.ID, getLevelForUser(targetUser.ID, guild.ID))
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
								models.EventlogTargetTypeUser, msg.Author.ID,
								models.EventlogTypeRobyulLevelsRoleGrant, "",
								nil,
								[]models.ElasticEventlogOption{
									{
										Key:   "levels_role_grant_roleid_removed",
										Value: targetRole.ID,
										Type:  models.EventlogTargetTypeRole,
									},
								}, false)
							helpers.RelaxLog(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.roles-grant-remove-success",
								targetUser.Username, targetUser.ID, targetRole.Name, targetRole.ID))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						_, err = m.createLevelsRolesOverwriteEntry(guild.ID, targetRole.ID, targetUser.ID, "grant")
						helpers.Relax(err)

						err = applyLevelsRoles(guild.ID, targetUser.ID, getLevelForUser(targetUser.ID, guild.ID))
						helpers.Relax(err)

						_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
							models.EventlogTargetTypeUser, msg.Author.ID,
							models.EventlogTypeRobyulLevelsRoleGrant, "",
							nil,
							[]models.ElasticEventlogOption{
								{
									Key:   "levels_role_grant_roleid_added",
									Value: targetRole.ID,
									Type:  models.EventlogTargetTypeRole,
								},
							}, false)
						helpers.RelaxLog(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.roles-grant-create-success",
							targetUser.Username, targetUser.ID, targetRole.Name, targetRole.ID))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					})
					return
				case "deny":
					// [p]levels roles deny <@user or user id> <role name or id>
					helpers.RequireMod(msg, func() {
						if len(args) < 4 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						guild, err := helpers.GetGuild(channel.GuildID)
						helpers.Relax(err)

						targetUser, err := helpers.GetUserFromMention(args[2])
						if err != nil || targetUser == nil || targetUser.ID == "" {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						var targetRole *discordgo.Role

						roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:3], " "), "", 1))
						for _, guildRole := range guild.Roles {
							if strings.ToLower(guildRole.Name) == strings.ToLower(roleNameToMatch) || guildRole.ID == roleNameToMatch {
								targetRole = guildRole
							}
						}

						if targetRole == nil || targetRole.ID == "" {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						previousGrant, previousDeny, grant := m.getLevelsRolesUserRoleOverwrite(guild.ID, targetRole.ID, targetUser.ID)

						if previousGrant {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.roles-deny-error-granting"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						if previousDeny {
							err = m.deleteLevelsRolesOverwriteEntry(grant)

							err = applyLevelsRoles(guild.ID, targetUser.ID, getLevelForUser(targetUser.ID, guild.ID))
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
								models.EventlogTargetTypeUser, msg.Author.ID,
								models.EventlogTypeRobyulLevelsRoleDeny, "",
								nil,
								[]models.ElasticEventlogOption{
									{
										Key:   "levels_role_deny_roleid_removed",
										Value: targetRole.ID,
										Type:  models.EventlogTargetTypeRole,
									},
								}, false)
							helpers.RelaxLog(err)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.roles-deny-remove-success",
								targetUser.Username, targetUser.ID, targetRole.Name, targetRole.ID))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						_, err = m.createLevelsRolesOverwriteEntry(guild.ID, targetRole.ID, targetUser.ID, "deny")
						helpers.Relax(err)

						err = applyLevelsRoles(guild.ID, targetUser.ID, getLevelForUser(targetUser.ID, guild.ID))
						helpers.Relax(err)

						_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
							models.EventlogTargetTypeUser, msg.Author.ID,
							models.EventlogTypeRobyulLevelsRoleDeny, "",
							nil,
							[]models.ElasticEventlogOption{
								{
									Key:   "levels_role_deny_roleid_added",
									Value: targetRole.ID,
									Type:  models.EventlogTargetTypeRole,
								},
							}, false)
						helpers.RelaxLog(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.roles-deny-create-success",
							targetUser.Username, targetUser.ID, targetRole.Name, targetRole.ID))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					})
					return
				}

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "set-level-notification", "set-level-notifications", "set-level-noti", "set-level-notis":
				helpers.RequireMod(msg, func() {
					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					var message string

					guildConfig := helpers.GuildSettingsGetCached(channel.GuildID)

					if len(args) >= 3 {
						embedCode := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
						if embedCode != "" {
							guildConfig.LevelsNotificationCode = embedCode
							message = helpers.GetText("plugins.levels.level-notification-enabled")
						}
					}

					if message == "" {
						guildConfig.LevelsNotificationCode = ""
						message = helpers.GetText("plugins.levels.level-notification-disabled")
					}

					err = helpers.GuildSettingsSet(channel.GuildID, guildConfig)
					helpers.Relax(err)

					_, err = helpers.SendMessage(msg.ChannelID, message)
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				})
				return
			case "set-level-notification-autodelete", "set-level-notifications-autodelete", "set-level-noti-autodelete", "set-level-notis-autodelete":
				helpers.RequireMod(msg, func() {
					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					var message string

					guildConfig := helpers.GuildSettingsGetCached(channel.GuildID)

					if len(args) >= 2 {
						deleteAfterN, err := strconv.Atoi(args[1])
						if err == nil && deleteAfterN > 0 {
							guildConfig.LevelsNotificationDeleteAfter = deleteAfterN
							message = helpers.GetTextF("plugins.levels.level-notification-autodelete-enabled", deleteAfterN)
						}
					}

					if message == "" {
						guildConfig.LevelsNotificationDeleteAfter = 0
						message = helpers.GetText("plugins.levels.level-notification-autodelete-disabled")
					}

					err = helpers.GuildSettingsSet(channel.GuildID, guildConfig)
					helpers.Relax(err)

					_, err = helpers.SendMessage(msg.ChannelID, message)
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				})
				return
			}
			targetUser, err = helpers.GetUserFromMention(args[0])
			if targetUser == nil || targetUser.ID == "" {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}
		}

		var levelsServersUser []models.LevelsServerusersEntry
		err = helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"userid": targetUser.ID})).All(&levelsServersUser)
		helpers.Relax(err)

		if levelsServersUser == nil {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.level-no-stats"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		} else if err != nil {
			helpers.Relax(err)
		}

		var levelThisServerUser models.LevelsServerusersEntry
		var totalExp int64
		for _, levelsServerUser := range levelsServersUser {
			if levelsServerUser.GuildID == channel.GuildID {
				levelThisServerUser = levelsServerUser
			}
			totalExp += levelsServerUser.Exp
		}

		if totalExp <= 0 {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.level-no-stats"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}

		currentMember, _ := helpers.GetGuildMember(channel.GuildID, targetUser.ID)
		if currentMember == nil || currentMember.User == nil || currentMember.User.ID == "" {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}

		fullUsername := currentMember.User.Username
		if currentMember.Nick != "" {
			fullUsername += " ~ " + currentMember.Nick
		}

		zeroWidthWhitespace, err := strconv.Unquote(`'\u200b'`)
		helpers.Relax(err)

		localExpForLevel := getExpForLevel(getLevelFromExp(levelThisServerUser.Exp))
		globalExpForLevel := getExpForLevel(getLevelFromExp(totalExp))

		userLevelEmbed := &discordgo.MessageEmbed{
			Color:       0x0FADED,
			Title:       helpers.GetTextF("plugins.levels.user-embed-title", fullUsername),
			Description: "View the leaderboard for this server [here](" + helpers.GetConfig().Path("website.ranking_base_url").Data().(string) + "/" + channel.GuildID + ").",
			Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.levels.embed-footer",
				len(session.State.Guilds),
			)},
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Level",
					Value:  strconv.Itoa(getLevelFromExp(levelThisServerUser.Exp)),
					Inline: true,
				},
				{
					Name: "Level Progress",
					Value: fmt.Sprintf("%s/%s EXP (%d %%)",
						humanize.Comma(levelThisServerUser.Exp-localExpForLevel), humanize.Comma(getExpForLevel(getLevelFromExp(levelThisServerUser.Exp)+1)-localExpForLevel),
						getProgressToNextLevelFromExp(levelThisServerUser.Exp),
					),
					Inline: true,
				},
				{
					Name:   zeroWidthWhitespace,
					Value:  zeroWidthWhitespace,
					Inline: true,
				},
				{
					Name:   "Global Level",
					Value:  strconv.Itoa(getLevelFromExp(totalExp)),
					Inline: true,
				},
				{
					Name: "Global Level Progress",
					Value: fmt.Sprintf("%s/%s EXP (%d %%)",
						humanize.Comma(totalExp-globalExpForLevel), humanize.Comma(getExpForLevel(getLevelFromExp(totalExp)+1)-globalExpForLevel),
						getProgressToNextLevelFromExp(totalExp),
					),
					Inline: true,
				},
				{
					Name:   zeroWidthWhitespace,
					Value:  zeroWidthWhitespace,
					Inline: true,
				},
			},
		}

		_, err = helpers.SendEmbed(msg.ChannelID, userLevelEmbed)
		helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		return
	case "leaderboard", "leaderboards", "ranking", "rankings":
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		link := helpers.GetConfig().Path("website.ranking_base_url").Data().(string) + "/" + channel.GuildID

		_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.ranking-text", link))
		helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		return
	}
}

func (l *Levels) DeleteMessages(channelID string, messages []string) {
	for _, message := range messages {
		cache.GetSession().ChannelMessageDelete(channelID, message)
	}
}

func (l *Levels) BadgePickerPrintCategories(user *discordgo.User, channeID string, availableBadges []models.ProfileBadgeEntry, activeBadgeIDs []string, allBadges []models.ProfileBadgeEntry) []string {
	categoriesCount := make(map[string]int, 0)
	for _, badge := range availableBadges {
		if _, ok := categoriesCount[badge.Category]; ok {
			categoriesCount[badge.Category] += 1
		} else {
			categoriesCount[badge.Category] = 1
		}
	}

	sortedKeys := make([]string, len(categoriesCount))
	i := 0
	for k := range categoriesCount {
		sortedKeys[i] = k
		i++
	}
	sort.Strings(sortedKeys)

	resultText := l.BadgePickerActiveText(user.Username, activeBadgeIDs, allBadges)
	resultText += "Choose a category name:\n"
	for _, key := range sortedKeys {
		resultText += fmt.Sprintf("__%s__ (%d badges)\n", key, categoriesCount[key])
	}
	resultText += l.BadgePickerHelpText()

	messageIDs := make([]string, 0)
	for _, page := range helpers.Pagify(resultText, "\n") {
		messages, err := helpers.SendMessage(channeID, page)
		helpers.RelaxMessage(err, channeID, "")
		for _, message := range messages {
			messageIDs = append(messageIDs, message.ID)
		}
	}
	return messageIDs
}

func (l *Levels) BadgePickerPrintBadges(user *discordgo.User, channeID string, availableBadges []models.ProfileBadgeEntry, activeBadgeIDs []string, categoryName string, allBadges []models.ProfileBadgeEntry) []string {
	categoryName = strings.ToLower(categoryName)

	resultText := l.BadgePickerActiveText(user.Username, activeBadgeIDs, allBadges)
	resultText += "Choose a badge name:\n"
	for _, badge := range availableBadges {
		if badge.Category == categoryName {
			isActive := false
			for _, activeBadgeID := range activeBadgeIDs {
				if activeBadgeID == badge.GetID() {
					isActive = true
				}
			}
			activeText := ""
			if isActive {
				activeText = " _**in use**_"
			}
			resultText += fmt.Sprintf("__%s__%s\n", badge.Name, activeText)
		}
	}
	resultText += l.BadgePickerHelpText()

	messageIDs := make([]string, 0)
	for _, page := range helpers.Pagify(resultText, "\n") {
		messages, err := helpers.SendMessage(channeID, page)
		helpers.RelaxMessage(err, channeID, "")
		for _, message := range messages {
			messageIDs = append(messageIDs, message.ID)
		}
	}
	return messageIDs
}

func (l *Levels) BadgePickerActiveText(username string, activeBadgeIDs []string, availableBadges []models.ProfileBadgeEntry) string {
	spaceLeft := BadgeLimt - len(activeBadgeIDs)
	text := fmt.Sprintf("**@%s** You can pick %d more badge(s) to display on your profile.\nYou are currently displaying:", username, spaceLeft)
	if len(activeBadgeIDs) > 0 {
		for _, badgeID := range activeBadgeIDs {
			for _, badge := range availableBadges {
				if badge.GetID() == badgeID {
					text += fmt.Sprintf(" `%s (%s)`", badge.Name, badge.Category)
				}
			}
		}
	} else {
		text += " No badges"
	}
	text += ".\n\n"
	return text
}

func (l *Levels) BadgePickerHelpText() string {
	return "\nSay `categories` to display all categories, `category name` to choose a category, `badge name` to choose a badge, `reset` to remove all badges displayed on your profile, `exit` to exit and save. To remove a badge from your Profile pick the badge again.\n"
}

func (l *Levels) GetUserUserdata(user *discordgo.User) (userdata models.ProfileUserdataEntry, err error) {
	err = helpers.MdbOne(
		helpers.MdbCollection(models.ProfileUserdataTable).Find(bson.M{"userid": user.ID}),
		&userdata,
	)

	if err == mgo.ErrNotFound {
		userdata.UserID = user.ID
		newid, err := helpers.MDbInsert(models.ProfileUserdataTable, userdata)
		userdata.ID = newid
		return userdata, err
	}

	return userdata, err
}

func (m *Levels) GetProfileHTML(member *discordgo.Member, guild *discordgo.Guild, web bool) (string, error) {
	var levelsServersUser []models.LevelsServerusersEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"userid": member.User.ID})).All(&levelsServersUser)
	if err != nil || levelsServersUser == nil {
		return "", err
	}

	var levelThisServerUser models.LevelsServerusersEntry
	var totalExp int64
	for _, levelsServerUser := range levelsServersUser {
		if levelsServerUser.GuildID == guild.ID {
			levelThisServerUser = levelsServerUser
		}
		totalExp += levelsServerUser.Exp
	}

	serverRank := "N/A"
	globalRank := "N/A"
	for _, serverCache := range topCache {
		if serverCache.GuildID == "global" {
			for i, pair := range serverCache.Levels {
				if pair.Key == member.User.ID {
					globalRank = strconv.Itoa(i + 1)
				}
			}
		} else if serverCache.GuildID == guild.ID {
			for i, pair := range serverCache.Levels {
				if pair.Key == member.User.ID {
					serverRank = strconv.Itoa(i + 1)
				}
			}
		}
	}

	userData, err := m.GetUserUserdata(member.User)
	if err != nil {
		return "", err
	}

	avatarUrl := helpers.GetAvatarUrl(member.User)
	avatarUrlGif := ""
	if avatarUrl != "" {
		avatarUrl = strings.Replace(avatarUrl, "size=1024", "size=128", -1)
		if strings.Contains(avatarUrl, "gif") {
			avatarUrlGif = avatarUrl
		}
		avatarUrl = strings.Replace(avatarUrl, "gif", "png", -1)
		avatarUrl = strings.Replace(avatarUrl, "jpg", "png", -1)
	}
	if web == true && avatarUrlGif != "" {
		avatarUrl = avatarUrlGif
	}
	if avatarUrl == "" {
		avatarUrl = "http://i.imgur.com/osAqNL6.png"
	}
	userAndNick := member.User.Username
	if member.Nick != "" {
		userAndNick = fmt.Sprintf("%s (%s)", member.User.Username, member.Nick)
	}
	userWithDisc := member.User.Username + "#" + member.User.Discriminator
	if helpers.RuneLength(userWithDisc) >= 15 {
		userWithDisc = member.User.Username
	}
	title := userData.Title
	if title == "" {
		title = "Robyul's friend"
	}
	bio := userData.Bio
	if bio == "" {
		bio = "Robyul would like to know more about me!"
	}

	badgesToDisplay := make([]models.ProfileBadgeEntry, 0)
	availableBadges := getBadgesAvailableQuick(member.User, userData.ActiveBadgeIDs)
	for _, activeBadgeID := range userData.ActiveBadgeIDs {
		for _, availableBadge := range availableBadges {
			if activeBadgeID == availableBadge.GetID() {
				badgesToDisplay = append(badgesToDisplay, availableBadge)
			}
		}
	}
	var badgesHTML1, badgesHTML2 string
	for i, badge := range badgesToDisplay {
		if i <= 8 {
			badgesHTML1 += fmt.Sprintf("<img src=\"%s\" style=\"border: 2px solid #%s;\">", getBadgeUrl(badge), badge.BorderColor)
		} else {
			badgesHTML2 += fmt.Sprintf("<img src=\"%s\" style=\"border: 2px solid #%s;\">", getBadgeUrl(badge), badge.BorderColor)
		}
	}

	backgroundColor, err := colorful.Hex("#" + m.GetBackgroundColor(userData))
	if err != nil {
		backgroundColor, err = colorful.Hex("#000000")
		if err != nil {
			return "", err
		}
	}
	backgroundColorString := fmt.Sprintf("rgba(%d, %d, %d, %s)",
		int(backgroundColor.R*255), int(backgroundColor.G*255), int(backgroundColor.B*255),
		m.GetBackgroundOpacity(userData))
	detailColorString := fmt.Sprintf("rgba(0, 0, 0, %s)",
		m.GetDetailOpacity(userData))

	userTimeText := ""
	if userData.Timezone != "" {
		userLocation, err := time.LoadLocation(userData.Timezone)
		if err == nil {
			userTimeText = "<i class=\"fa fa-clock-o\" aria-hidden=\"true\"></i> " + time.Now().In(userLocation).Format(TimeAtUserFormat)
		}
	}

	userBirthdayText := ""
	isBirthday := false
	if userData.Birthday != "" {
		userLocation, err := time.LoadLocation("Etc/UTC")
		if err == nil {
			if userData.Timezone != "" {
				userLocationUser, err := time.LoadLocation(userData.Timezone)
				if err == nil {
					userLocation = userLocationUser
				}
			}
			birthdayTime, err := time.ParseInLocation(TimeBirthdayFormat, userData.Birthday, userLocation)
			birthdayTime = birthdayTime.AddDate(time.Now().Year(), 0, 0)
			if err == nil {
				if time.Now().In(userLocation).Sub(birthdayTime).Hours() <= 23 && time.Now().In(userLocation).Sub(birthdayTime).Hours() > 0 {
					isBirthday = true
				}
			}
		}

		userBirthdayText = "<i class=\"fa fa-birthday-cake\" aria-hidden=\"true\"></i> " + userData.Birthday
		if isBirthday {
			userBirthdayText = "<i class=\"fa fa-birthday-cake\" aria-hidden=\"true\"></i> Today!"
		}
	}

	var playingStatus string
	if !userData.HideLastFm {
		lastfmUsername := helpers.GetLastFmUsername(member.User.ID)
		if lastfmUsername != "" {
			recentTracks, err := helpers.GetLastFmClient().User.GetRecentTracks(lastfm.P{
				"limit": 1,
				"user":  lastfmUsername,
			})
			if err != nil && !strings.Contains(err.Error(), "User not found") {
				helpers.RelaxLog(err)
			}
			if err == nil && recentTracks.Tracks != nil && len(recentTracks.Tracks) >= 1 && recentTracks.Tracks[0].NowPlaying == "true" {
				playingStatus += fmt.Sprintf("<i class=\"fa fa-music\" aria-hidden=\"true\"></i> %s by %s",
					recentTracks.Tracks[0].Name, recentTracks.Tracks[0].Artist.Name)
			}
			topArtists, err := helpers.GetLastFmClient().User.GetTopArtists(lastfm.P{
				"limit":  1,
				"period": "overall",
				"user":   lastfmUsername,
			})
			if err != nil && !strings.Contains(err.Error(), "User not found") {
				helpers.RelaxLog(err)
			}
			if err == nil && topArtists.Artists != nil && len(topArtists.Artists) >= 1 {
				if playingStatus != "" {
					playingStatus += "<br>"
				}
				playCountN, err := strconv.Atoi(topArtists.Artists[0].PlayCount)
				helpers.RelaxLog(err)
				if err == nil {
					playingStatus += fmt.Sprintf("<i class=\"fa fa-users\" aria-hidden=\"true\"></i> %s",
						topArtists.Artists[0].Name)
					playCountText := fmt.Sprintf("(%s plays)", humanize.Comma(int64(playCountN)))
					if helpers.RuneLength(topArtists.Artists[0].Name)+1+helpers.RuneLength(playCountText) <= 20 {
						playingStatus += " " + playCountText
					}
				}
			}
		}
	}

	expOpacity := m.GetExpOpacity(userData)
	badgeOpacity := m.GetBadgeOpacity(userData)

	tempTemplateHtml := strings.Replace(htmlTemplateString, "{USER_USERNAME}", html.EscapeString(member.User.Username), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_NICKNAME}", html.EscapeString(member.Nick), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_AND_NICKNAME}", html.EscapeString(userAndNick), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USERNAME_WITH_DISC}", html.EscapeString(userWithDisc), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_AVATAR_URL}", html.EscapeString(avatarUrl), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TITLE}", html.EscapeString(title), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BIO}", html.EscapeString(bio), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_LEVEL}", strconv.Itoa(getLevelFromExp(levelThisServerUser.Exp)), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_RANK}", serverRank, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_LEVEL_PERCENT}", strconv.Itoa(getProgressToNextLevelFromExp(levelThisServerUser.Exp)), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_GLOBAL_LEVEL}", strconv.Itoa(getLevelFromExp(totalExp)), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_GLOBAL_RANK}", globalRank, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BACKGROUND_URL}", m.GetProfileBackgroundUrl(userData), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_REP}", strconv.Itoa(userData.Rep), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BADGES_HTML_1}", badgesHTML1, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BADGES_HTML_2}", badgesHTML2, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BACKGROUND_COLOR}", html.EscapeString(backgroundColorString), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_ACCENT_COLOR}", "#"+m.GetAccentColor(userData), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_DETAIL_COLOR}", html.EscapeString(detailColorString), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TEXT_COLOR}", "#"+m.GetTextColor(userData), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_EXP_OPACITY}", expOpacity, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BADGE_OPACITY}", badgeOpacity, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_PLAYING}", playingStatus, -1)

	if web == false { // privacy
		tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TIME}", userTimeText, -1)
		tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BIRTHDAY}", userBirthdayText, -1)
	} else {
		tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TIME}", "", -1)
		tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BIRTHDAY}", "", -1)
	}

	return tempTemplateHtml, nil
}

func (m *Levels) GetProfile(member *discordgo.Member, guild *discordgo.Guild, gifP bool) ([]byte, string, error) {
	tempTemplateHtml, err := m.GetProfileHTML(member, guild, false)
	if err != nil {
		return []byte{}, "", err
	}

	start := time.Now()
	imageBytes, err := helpers.TakeHTMLScreenshot(tempTemplateHtml, 400, 300)
	if err != nil {
		return []byte{}, "", err
	}
	elapsed := time.Since(start)
	cache.GetLogger().WithField("module", "levels").Info(fmt.Sprintf("took screenshot of profile in %s", elapsed.String()))

	metrics.LevelImagesGenerated.Add(1)

	avatarUrlGif := helpers.GetAvatarUrl(member.User)
	if avatarUrlGif != "" {
		avatarUrlGif = strings.Replace(avatarUrlGif, "size=1024", "size=128", -1)
		if !strings.Contains(avatarUrlGif, "gif") {
			avatarUrlGif = ""
		}
	}

	if avatarUrlGif != "" && gifP == true {
		outGif := &gif.GIF{}

		decodedFirstFrame, err := png.Decode(bytes.NewReader(imageBytes))
		if err != nil {
			raven.SetUserContext(&raven.User{
				Username: member.User.Username + "#" + member.User.Discriminator,
			})
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			goto ReturnImageBytes
		}
		buf := bytes.Buffer{}
		err = gif.Encode(&buf, decodedFirstFrame, nil)
		if err != nil {
			raven.SetUserContext(&raven.User{
				Username: member.User.Username + "#" + member.User.Discriminator,
			})
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			goto ReturnImageBytes
		}
		firstFrame, err := gif.Decode(&buf)
		if err != nil {
			raven.SetUserContext(&raven.User{
				Username: member.User.Username + "#" + member.User.Discriminator,
			})
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			goto ReturnImageBytes
		}

		avatarGifBytes, err := helpers.NetGetUAWithError(avatarUrlGif, helpers.DEFAULT_UA)
		if err != nil {
			raven.SetUserContext(&raven.User{
				Username: member.User.Username + "#" + member.User.Discriminator,
			})
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			goto ReturnImageBytes
		}

		avatarGif, err := gif.DecodeAll(bytes.NewReader(avatarGifBytes))
		if err != nil {
			raven.SetUserContext(&raven.User{
				Username: member.User.Username + "#" + member.User.Discriminator,
			})
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			goto ReturnImageBytes
		}

		fullRect := image.Rect(0, 0, 400, 300)
		pm := image.NewPaletted(fullRect, nil)
		q := gogif.MedianCutQuantizer{NumColor: 256}
		q.Quantize(pm, fullRect, decodedFirstFrame, image.ZP)
		draw.FloydSteinberg.Draw(pm, fullRect, firstFrame, image.ZP)

		outGif.Image = append(outGif.Image, pm)
		outGif.Delay = append(outGif.Delay, avatarGif.Delay[0])

		resizeRect := image.Rect(0, 0, 128, 128)
		cutImage := image.NewRGBA(resizeRect)
		resizedRect := image.Rect(4, 64+90, 4+80, 64+80+90)

		for i, avatarGifFrame := range avatarGif.Image {
			resizedImage := resize.Resize(80, 80, avatarGifFrame, resize.NearestNeighbor)
			draw.DrawMask(
				cutImage, resizedImage.Bounds(), resizedImage, image.ZP,
				&circle{image.Pt(40, 40), 40}, image.ZP, draw.Over)
			avatarGifFrame.Palette = append(avatarGifFrame.Palette, image.Transparent)
			paletteHasTransparency := false
			newPalette := make([]color.Color, 0)
			for i, color := range avatarGifFrame.Palette {
				if color == image.Transparent {
					paletteHasTransparency = true
				}
				newPalette = append(newPalette, color)
				if i == 254 && paletteHasTransparency == false {
					newPalette = append(newPalette, image.Transparent)
					break
				}
			}
			pm = image.NewPaletted(resizedRect, newPalette)
			draw.FloydSteinberg.Draw(pm, resizedRect, cutImage, image.ZP)
			outGif.Image = append(outGif.Image, pm)
			outGif.Delay = append(outGif.Delay, avatarGif.Delay[i])
		}

		outGif.LoopCount = -1

		buf = bytes.Buffer{}
		err = gif.EncodeAll(&buf, outGif)
		if err != nil {
			raven.SetUserContext(&raven.User{
				Username: member.User.Username + "#" + member.User.Discriminator,
			})
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			goto ReturnImageBytes
		}
		return buf.Bytes(), "gif", nil
	}
ReturnImageBytes:
	return imageBytes, "png", nil
}

type circle struct {
	p image.Point
	r int
}

func (c *circle) ColorModel() color.Model {
	return color.AlphaModel
}

func (c *circle) Bounds() image.Rectangle {
	return image.Rect(c.p.X-c.r, c.p.Y-c.r, c.p.X+c.r, c.p.Y+c.r)
}

func (c *circle) At(x, y int) color.Color {
	xx, yy, rr := float64(x-c.p.X)+0.5, float64(y-c.p.Y)+0.5, float64(c.r)
	if xx*xx+yy*yy < rr*rr {
		return color.Alpha{255}
	}
	return color.Alpha{0}
}

func (m *Levels) GetBackgroundColor(userUserdata models.ProfileUserdataEntry) string {
	if userUserdata.BackgroundColor != "" {
		return userUserdata.BackgroundColor
	} else {
		return "000000"
	}
}

func (m *Levels) GetAccentColor(userUserdata models.ProfileUserdataEntry) string {
	if userUserdata.AccentColor != "" {
		return userUserdata.AccentColor
	} else {
		return "46d42e"
	}
}

func (m *Levels) GetTextColor(userUserdata models.ProfileUserdataEntry) string {
	if userUserdata.TextColor != "" {
		return userUserdata.TextColor
	} else {
		return "ffffff"
	}
}

func (m *Levels) GetBackgroundOpacity(userUserdata models.ProfileUserdataEntry) string {
	if userUserdata.BackgroundOpacity != "" {
		return userUserdata.BackgroundOpacity
	} else {
		return "0.5"
	}
}

func (m *Levels) GetDetailOpacity(userUserdata models.ProfileUserdataEntry) string {
	if userUserdata.DetailOpacity != "" {
		return userUserdata.DetailOpacity
	} else {
		return "0.5"
	}
}

func (m *Levels) GetExpOpacity(userUserdata models.ProfileUserdataEntry) string {
	if userUserdata.DetailOpacity != "" {
		return userUserdata.EXPOpacity
	} else {
		return "0.5"
	}
}

func (m *Levels) GetBadgeOpacity(userUserdata models.ProfileUserdataEntry) string {
	if userUserdata.BadgeOpacity != "" {
		return userUserdata.BadgeOpacity
	} else {
		return "1.0"
	}
}

func (m *Levels) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
	channel, err := helpers.GetChannel(msg.ChannelID)
	helpers.Relax(err)

	if helpers.IsLimitedGuild(channel.GuildID) {
		return
	}

	go m.ProcessMessage(msg, session)
}

func (m *Levels) ProcessMessage(msg *discordgo.Message, session *discordgo.Session) {
	channel, err := helpers.GetChannel(msg.ChannelID)
	helpers.Relax(err)
	// ignore temporary ignored guilds
	for _, temporaryIgnoredGuild := range temporaryIgnoredGuilds {
		if temporaryIgnoredGuild == channel.GuildID {
			return
		}
	}
	// ignore bot messages
	if msg.Author.Bot == true {
		return
	}
	// ignore commands
	prefix := helpers.GetPrefixForServer(channel.GuildID)
	if prefix != "" {
		if strings.HasPrefix(msg.Content, prefix) {
			return
		}
	}

	settings := helpers.GuildSettingsGetCached(channel.GuildID)
	for _, ignoredChannelID := range settings.LevelsIgnoredChannelIDs {
		if ignoredChannelID == msg.ChannelID {
			return
		}
	}
	for _, ignoredUserID := range settings.LevelsIgnoredUserIDs {
		for ignoredUserID == msg.Author.ID {
			return
		}
	}

	// check if bucket is empty
	if !m.BucketHasKeys(channel.GuildID + msg.Author.ID) {
		//m.BucketSet(channel.GuildID+msg.Author.ID, -1)
		return
	}

	err = m.BucketDrain(1, channel.GuildID+msg.Author.ID)
	helpers.Relax(err)

	expStack.Push(ProcessExpInfo{UserID: msg.Author.ID, GuildID: channel.GuildID, ChannelID: msg.ChannelID})
}

func (m *Levels) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		if member.User == nil {
			return
		}

		err := applyLevelsRoles(member.GuildID, member.User.ID, getLevelForUser(member.User.ID, member.GuildID))
		if err != nil {
			if errD, ok := err.(*discordgo.RESTError); !ok || (errD.Message.Code != discordgo.ErrCodeUnknownMember && errD.Message.Code != discordgo.ErrCodeMissingAccess) {
				helpers.RelaxLog(err)
			}
		}
	}()

}

func (m *Levels) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Levels) getLevelsServerUserOrCreateNew(guildid string, userid string) (serveruser models.LevelsServerusersEntry, err error) {
	err = helpers.MdbOne(
		helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"userid": userid, "guildid": guildid}),
		&serveruser,
	)

	if err == mgo.ErrNotFound {
		serveruser.UserID = userid
		serveruser.GuildID = guildid
		newid, err := helpers.MDbInsert(models.LevelsServerusersTable, serveruser)
		serveruser.ID = newid
		return serveruser, err
	}

	return serveruser, err
}

// Refills user buckets in a set interval
func (b *Levels) BucketRefiller() {
	for {
		b.Lock()
		for user, keys := range b.buckets {
			// Chill zone
			if keys == -1 {
				b.buckets[user]++
				continue
			}

			// Chill zone exit
			if keys == 0 {
				b.buckets[user] = BUCKET_INITIAL_FILL
				continue
			}

			// More free keys for nice users :3
			if keys < BUCKET_UPPER_BOUND {
				b.buckets[user] += DROP_SIZE
				continue
			}
		}
		b.Unlock()

		time.Sleep(DROP_INTERVAL)
	}
}

// Check if the user has a bucket. If not create one
func (b *Levels) CreateBucketIfNotExists(user string) {
	if b.buckets == nil {
		return
	}

	b.RLock()
	_, e := b.buckets[user]
	b.RUnlock()

	if !e {
		b.Lock()
		b.buckets[user] = BUCKET_INITIAL_FILL
		b.Unlock()
	}
}

// Drains $amount from $user if he has enough keys left
func (b *Levels) BucketDrain(amount int8, user string) error {
	b.CreateBucketIfNotExists(user)

	// Check if there are enough keys left
	b.RLock()
	userAmount := b.buckets[user]
	b.RUnlock()

	if amount > userAmount {
		return errors.New("No keys left")
	}

	// Remove keys from bucket
	b.Lock()
	b.buckets[user] -= amount
	b.Unlock()

	return nil
}

// Check if the user still has keys
func (b *Levels) BucketHasKeys(user string) bool {
	b.CreateBucketIfNotExists(user)

	b.RLock()
	defer b.RUnlock()

	return b.buckets[user] > 0
}

func (b *Levels) BucketGet(user string) int8 {
	b.RLock()
	defer b.RUnlock()

	return b.buckets[user]
}

func (b *Levels) BucketSet(user string, value int8) {
	if b.buckets == nil {
		return
	}

	b.Lock()
	b.buckets[user] = value
	b.Unlock()
}

func (b *Levels) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (b *Levels) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (b *Levels) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (b *Levels) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (b *Levels) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (l *Levels) createLevelsRoleEntry(
	guildID string,
	roleID string,
	startLevel int,
	lastLevel int,
) (result models.LevelsRoleEntry, err error) {
	insert := rethink.Table(models.LevelsRolesTable).Insert(models.LevelsRoleEntry{
		GuildID:    guildID,
		RoleID:     roleID,
		StartLevel: startLevel,
		LastLevel:  lastLevel,
	})
	inserted, err := insert.RunWrite(helpers.GetDB())
	if err != nil {
		return models.LevelsRoleEntry{}, err
	} else {
		return l.getLevelsRoleEntryBy("id", inserted.GeneratedKeys[0])
	}
}

func (l *Levels) getLevelsRoleEntryBy(key string, value string) (result models.LevelsRoleEntry, err error) {
	listCursor, err := rethink.Table(models.LevelsRolesTable).Filter(
		rethink.Row.Field(key).Eq(value),
	).Run(helpers.GetDB())
	if err != nil {
		return result, err
	}
	defer listCursor.Close()
	err = listCursor.One(&result)

	if err == rethink.ErrEmptyResult {
		return result, errors.New("no levels role entry")
	}

	return result, err
}

func (l *Levels) getLevelsRoleEntriesBy(key string, value string) (result []models.LevelsRoleEntry, err error) {
	listCursor, err := rethink.Table(models.LevelsRolesTable).Filter(
		rethink.Row.Field(key).Eq(value),
	).Run(helpers.GetDB())
	if err != nil {
		return result, err
	}
	defer listCursor.Close()
	err = listCursor.All(&result)

	if err == rethink.ErrEmptyResult {
		return result, errors.New("no levels role entries")
	}

	return result, err
}

func (l *Levels) deleteLevelsRoleEntry(levelsRoleEntry models.LevelsRoleEntry) (err error) {
	if levelsRoleEntry.ID != "" {
		_, err := rethink.Table(models.LevelsRolesTable).Get(levelsRoleEntry.ID).Delete().RunWrite(helpers.GetDB())
		return err
	}
	return errors.New("empty levelsRoleEntry submitted")
}

func (l *Levels) createLevelsRolesOverwriteEntry(
	guildID string,
	roleID string,
	userID string,
	overwriteType string,
) (result models.LevelsRoleOverwriteEntry, err error) {
	if overwriteType != "grant" && overwriteType != "deny" {
		return models.LevelsRoleOverwriteEntry{}, errors.New("invalid overwrite type")
	}

	insert := rethink.Table(models.LevelsRoleOverwritesTable).Insert(models.LevelsRoleOverwriteEntry{
		GuildID: guildID,
		RoleID:  roleID,
		UserID:  userID,
		Type:    overwriteType,
	})
	inserted, err := insert.RunWrite(helpers.GetDB())
	if err != nil {
		return models.LevelsRoleOverwriteEntry{}, err
	} else {
		return l.getLevelsRoleOverwriteEntryBy("id", inserted.GeneratedKeys[0])
	}
}

func (l *Levels) getLevelsRoleOverwriteEntryBy(key string, value string) (result models.LevelsRoleOverwriteEntry, err error) {
	listCursor, err := rethink.Table(models.LevelsRoleOverwritesTable).Filter(
		rethink.Row.Field(key).Eq(value),
	).Run(helpers.GetDB())
	if err != nil {
		return result, err
	}
	defer listCursor.Close()
	err = listCursor.One(&result)

	if err == rethink.ErrEmptyResult {
		return result, errors.New("no levels role overwrite entry")
	}

	return result, err
}

func (l *Levels) getLevelsRolesUserRoleOverwrite(guildID string, roleID string, userID string) (grant bool, deny bool, overwrite models.LevelsRoleOverwriteEntry) {
	listCursor, err := rethink.Table(models.LevelsRoleOverwritesTable).Filter(
		rethink.And(
			rethink.Row.Field("guild_id").Eq(guildID),
			rethink.Row.Field("user_id").Eq(userID),
			rethink.Row.Field("role_id").Eq(roleID),
		),
	).Run(helpers.GetDB())
	if err != nil {
		helpers.RelaxLog(err)
		return false, false, models.LevelsRoleOverwriteEntry{}
	}
	defer listCursor.Close()
	err = listCursor.One(&overwrite)

	if err == rethink.ErrEmptyResult {
		return false, false, models.LevelsRoleOverwriteEntry{}
	}

	switch overwrite.Type {
	case "grant":
		return true, false, overwrite
	case "deny":
		return false, true, overwrite
	}

	return false, false, overwrite
}

func (l *Levels) getLevelsRolesGuildOverwrites(guildID string) (overwrites []models.LevelsRoleOverwriteEntry) {
	listCursor, err := rethink.Table(models.LevelsRoleOverwritesTable).Filter(
		rethink.And(
			rethink.Row.Field("guild_id").Eq(guildID),
		),
	).Run(helpers.GetDB())
	if err != nil {
		helpers.RelaxLog(err)
		return make([]models.LevelsRoleOverwriteEntry, 0)
	}
	defer listCursor.Close()
	err = listCursor.All(&overwrites)

	if err == rethink.ErrEmptyResult {
		return make([]models.LevelsRoleOverwriteEntry, 0)
	}

	return
}

func (l *Levels) deleteLevelsRolesOverwriteEntry(levelsRolesOverwriteEntry models.LevelsRoleOverwriteEntry) (err error) {
	if levelsRolesOverwriteEntry.ID != "" {
		_, err = rethink.Table(models.LevelsRoleOverwritesTable).Get(levelsRolesOverwriteEntry.ID).Delete().RunWrite(helpers.GetDB())
		return err
	}
	return errors.New("empty levelsRoleOverwriteEntry submitted")
}

func (l *Levels) lockRepUser(userID string) {
	if _, ok := repCommandLocks[userID]; ok {
		repCommandLocks[userID].Lock()
		return
	}
	repCommandLocks[userID] = new(sync.Mutex)
	repCommandLocks[userID].Lock()
}

func (l *Levels) unlockRepUser(userID string) {
	if _, ok := repCommandLocks[userID]; ok {
		repCommandLocks[userID].Unlock()
	}
}

func (m *Levels) logUserBackgroundNotSafe(targetChannelID, sourceChannelID, userID, backgroundUrl string) (err error) {
	author, err := helpers.GetUser(userID)
	if err != nil {
		return err
	}

	channel, err := helpers.GetChannel(sourceChannelID)
	if err != nil {
		return err
	}

	guild, err := helpers.GetGuild(channel.GuildID)
	if err != nil {
		return err
	}

	targetChannel, err := helpers.GetChannel(targetChannelID)
	if err != nil {
		return err
	}

	_, err = helpers.SendEmbed(targetChannelID, &discordgo.MessageEmbed{
		URL:   backgroundUrl,
		Title: "Background got rejected because it's not safe ❌",
		Color: 0,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("In #%s (#%s) on %s (#%s)",
				channel.Name, channel.ID,
				guild.Name, guild.ID),
		},
		Image: &discordgo.MessageEmbedImage{
			URL: backgroundUrl,
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name:    author.Username + "#" + author.Discriminator + " (#" + author.ID + ")",
			IconURL: author.AvatarURL("64"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Override the background:",
				Value: fmt.Sprintf("`%sprofile background force %s %s`",
					helpers.GetPrefixForServer(targetChannel.GuildID),
					author.ID,
					backgroundUrl),
				Inline: false,
			},
		},
	})

	return nil
}

func (m *Levels) logUserBackgroundSet(targetChannelID, sourceChannelID, userID, backgroundUrl string) (err error) {
	author, err := helpers.GetUser(userID)
	if err != nil {
		return err
	}

	channel, err := helpers.GetChannel(sourceChannelID)
	if err != nil {
		return err
	}

	guild, err := helpers.GetGuild(channel.GuildID)
	if err != nil {
		return err
	}

	targetChannel, err := helpers.GetChannel(targetChannelID)
	if err != nil {
		return err
	}

	_, err = helpers.SendEmbed(targetChannelID, &discordgo.MessageEmbed{
		URL:   backgroundUrl,
		Title: "Background got accepted ✅",
		Color: 0,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("In #%s (#%s) on %s (#%s)",
				channel.Name, channel.ID,
				guild.Name, guild.ID),
		},
		Image: &discordgo.MessageEmbedImage{
			URL: backgroundUrl,
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name:    author.Username + "#" + author.Discriminator + " (#" + author.ID + ")",
			IconURL: author.AvatarURL("64"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Reset background:",
				Value: fmt.Sprintf("`%sprofile background reset %s`",
					helpers.GetPrefixForServer(targetChannel.GuildID),
					author.ID),
				Inline: false,
			},
			{
				Name: "Disable uploads for this user:",
				Value: fmt.Sprintf("`%suseruploads disable %s`",
					helpers.GetPrefixForServer(targetChannel.GuildID),
					author.ID),
				Inline: false,
			},
		},
	})

	return nil
}
