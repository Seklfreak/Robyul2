package plugins

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
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"regexp"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/ratelimits"
	"github.com/andybons/gogif"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/getsentry/raven-go"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	redisCache "github.com/go-redis/cache"
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
	GuildID string
	UserID  string
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

type DB_Profile_Background struct {
	Name      string    `gorethink:"id,omitempty"`
	URL       string    `gorethink:"url"`
	CreatedAt time.Time `gorethink:"createdat"`
	Tags      []string  `gorethink:"tags"`
}

type DB_Badge struct {
	ID               string    `gorethink:"id,omitempty"`
	CreatedByUserID  string    `gorethink:"createdby_userid"`
	Name             string    `gorethink:"name"`
	Category         string    `gorethink:"category"`
	BorderColor      string    `gorethink:"bordercolor"`
	GuildID          string    `gorethink:"guildid"`
	CreatedAt        time.Time `gorethink:"createdat"`
	URL              string    `gorethink:"url"`
	LevelRequirement int       `gorethink:"levelrequirement"`
	RoleRequirement  string    `gorethink:"rolerequirement"`
	AllowedUserIDs   []string  `gorethinK:"allowed_userids"`
	DeniedUserIDs    []string  `gorethinK:"allowed_userids"`
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
	webshotBinary            string
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
	webshotBinary, err = exec.LookPath("webshot")
	helpers.Relax(err)

	go m.processExpStackLoop()
	log.WithField("module", "levels").Info("Started processExpStackLoop")

	go m.cacheTopLoop()
	log.WithField("module", "levels").Info("Started processCacheTopLoop")

	activeBadgePickerUserIDs = make(map[string]string, 0)

	go m.setServerFeaturesLoop()
}

func (l *Levels) Uninit(session *discordgo.Session) {

}

func (l *Levels) setServerFeaturesLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "levels").Error("The setServerFeaturesLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			l.setServerFeaturesLoop()
		}()
	}()

	var badgesBucket []DB_Badge
	var badgesOnServer []DB_Badge
	var listCursor *rethink.Cursor
	var err error
	var feature models.Rest_Feature_Levels_Badges
	var key string
	cacheCodec := cache.GetRedisCacheCodec()
	for {
		listCursor, err = rethink.Table("profile_badge").Run(helpers.GetDB())
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			time.Sleep(60 * time.Second)
			continue
		}
		defer listCursor.Close()
		err = listCursor.All(&badgesBucket)
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			time.Sleep(60 * time.Second)
			continue
		}

		for _, guild := range cache.GetSession().State.Guilds {
			badgesOnServer = make([]DB_Badge, 0)
			for _, badge := range badgesBucket {
				if badge.GuildID == guild.ID {
					badgesOnServer = append(badgesOnServer, badge)
				}
			}

			key = fmt.Sprintf(models.Redis_Key_Feature_Levels_Badges, guild.ID)
			feature = models.Rest_Feature_Levels_Badges{
				Count: len(badgesOnServer),
			}

			err = cacheCodec.Set(&redisCache.Item{
				Key:        key,
				Object:     feature,
				Expiration: time.Minute * 60,
			})
			if err != nil {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			}

		}

		time.Sleep(30 * time.Minute)
	}
}

func (m *Levels) cacheTopLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "levels").Error("The cacheTopLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.cacheTopLoop()
		}()
	}()

	for {
		// TODO: cache still required with MongoDB?
		var newTopCache []Cache_Levels_top

		var levelsUsers []models.LevelsServerusersEntry

		err := helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(nil)).All(&levelsUsers)
		helpers.Relax(err)

		if levelsUsers == nil || len(levelsUsers) <= 0 {
			log.WithField("module", "levels").Error("empty result from levels db")
			time.Sleep(60 * time.Second)
			continue
		} else if err != nil {
			log.WithField("module", "levels").Error(fmt.Sprintf("db error: %s", err.Error()))
			time.Sleep(60 * time.Second)
			continue
		}

		for _, guild := range cache.GetSession().State.Guilds {
			guildExpMap := make(map[string]int64, 0)
			for _, levelsUser := range levelsUsers {
				if levelsUser.GuildID == guild.ID {
					guildExpMap[levelsUser.UserID] = levelsUser.Exp
				}
			}
			rankedGuildExpMap := m.rankMapByExp(guildExpMap)
			newTopCache = append(newTopCache, Cache_Levels_top{
				GuildID: guild.ID,
				Levels:  rankedGuildExpMap,
			})
		}

		totalExpMap := make(map[string]int64, 0)
		for _, levelsUser := range levelsUsers {
			if _, ok := totalExpMap[levelsUser.UserID]; ok {
				totalExpMap[levelsUser.UserID] += levelsUser.Exp
			} else {
				totalExpMap[levelsUser.UserID] = levelsUser.Exp
			}
		}

		rankedTotalExpMap := m.rankMapByExp(totalExpMap)
		newTopCache = append(newTopCache, Cache_Levels_top{
			GuildID: "global",
			Levels:  rankedTotalExpMap,
		})

		topCache = newTopCache

		var keyByRank string
		var keyByUser string
		var rankData Levels_Cache_Ranking_Item
		cacheCodec := cache.GetRedisCacheCodec()
		for _, guildCache := range newTopCache {
			i := 0
			for _, level := range guildCache.Levels {
				if level.Value > 0 {
					i += 1
					keyByRank = fmt.Sprintf("robyul2-discord:levels:ranking:%s:by-rank:%d", guildCache.GuildID, i)
					keyByUser = fmt.Sprintf("robyul2-discord:levels:ranking:%s:by-user:%s", guildCache.GuildID, level.Key)
					rankData = Levels_Cache_Ranking_Item{
						UserID:  level.Key,
						EXP:     level.Value,
						Level:   m.getLevelFromExp(level.Value),
						Ranking: i,
					}

					err = cacheCodec.Set(&redisCache.Item{
						Key:        keyByRank,
						Object:     &rankData,
						Expiration: 90 * time.Minute,
					})
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
					}
					err = cacheCodec.Set(&redisCache.Item{
						Key:        keyByUser,
						Object:     &rankData,
						Expiration: 90 * time.Minute,
					})
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
					}
				}
			}
			keyByRank = fmt.Sprintf("robyul2-discord:levels:ranking:%s:by-rank:count", guildCache.GuildID)
			keyByUser = fmt.Sprintf("robyul2-discord:levels:ranking:%s:by-user:count", guildCache.GuildID)
			err = cacheCodec.Set(&redisCache.Item{
				Key:        keyByRank,
				Object:     i,
				Expiration: 90 * time.Minute,
			})
			if err != nil {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			}
			err = cacheCodec.Set(&redisCache.Item{
				Key:        keyByUser,
				Object:     i,
				Expiration: 90 * time.Minute,
			})
			if err != nil {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			}
		}
		log.WithField("module", "levels").Info("cached rankings in redis")

		time.Sleep(10 * time.Minute)
	}
}

func (m *Levels) processExpStackLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "levels").Info("The processExpStackLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.processExpStackLoop()
		}()
	}()

	for {
		if !expStack.Empty() {
			expItem := expStack.Pop().(ProcessExpInfo)
			levelsServerUser, err := m.getLevelsServerUserOrCreateNewWithoutLogging(expItem.GuildID, expItem.UserID)
			helpers.Relax(err)

			expBefore := levelsServerUser.Exp
			levelBefore := m.getLevelFromExp(levelsServerUser.Exp)

			levelsServerUser.Exp += m.getRandomExpForMessage()

			levelAfter := m.getLevelFromExp(levelsServerUser.Exp)

			_, err = helpers.MDbUpdateWithoutLogging(models.LevelsServerusersTable, levelsServerUser.ID, levelsServerUser)
			helpers.Relax(err)

			if expBefore <= 0 || levelBefore != levelAfter {
				err := m.applyLevelsRoles(expItem.GuildID, expItem.UserID, levelAfter)
				if errD, ok := err.(*discordgo.RESTError); !ok || (errD.Message.Message != "404: Not Found" &&
					errD.Message.Code != discordgo.ErrCodeUnknownMember &&
					errD.Message.Code != discordgo.ErrCodeMissingAccess) {
					helpers.RelaxLog(err)
				}
			}
		} else {
			time.Sleep(1 * time.Second)
		}
	}
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
		_, err = helpers.MDbUpdate(models.ProfileUserdataTable, targetUserData.ID, targetUserData)
		helpers.Relax(err)

		userData.LastRepped = time.Now()
		_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
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
			case "title":
				titleText := " "
				if len(args) >= 2 {
					titleText = strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
				}

				userUserdata, err := m.GetUserUserdata(msg.Author)
				helpers.Relax(err)
				userUserdata.Title = titleText
				_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
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
				userUserdata.Bio = bioText
				_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-bio-set-success"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			case "background", "backgrounds":
				if len(args) < 2 {
					if len(msg.Attachments) <= 0 {
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.new-profile-background-help"))
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

					backgroundUrl, err := helpers.UploadImage(bytesData)
					if err != nil {
						helpers.RelaxLog(err)
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-background-upload-failed"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}

					userUserdata, err := m.GetUserUserdata(msg.Author)
					helpers.Relax(err)
					userUserdata.Background = backgroundUrl
					_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
					helpers.Relax(err)

					go func() {
						logChannelID, _ := helpers.GetBotConfigString(models.UserProfileBackgroundLogChannelKey)
						if logChannelID != "" {
							err = m.logUserBackgroundSet(logChannelID, msg.ChannelID, msg.Author.ID, backgroundUrl)
							helpers.RelaxLog(err)
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

						backgroundUrl, err := helpers.UploadImage(bytesData)
						if err != nil {
							helpers.RelaxLog(err)
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.user-background-upload-failed"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}

						userUserdata, err := m.GetUserUserdata(userToChange)
						helpers.Relax(err)
						userUserdata.Background = backgroundUrl
						_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
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
						_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
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
						backgroundUrl, err = helpers.UploadImage(picData)
						if err != nil {
							if strings.Contains(err.Error(), "Invalid URL") {
								_, err = helpers.SendMessage(msg.ChannelID, "I wasn't able to reupload the picture. Please make sure it is a direct link to the image.")
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							} else {
								helpers.Relax(err)
							}
							return
						}

						if m.ProfileBackgroundNameExists(backgroundName) == true {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.new-profile-background-add-error-duplicate"))
							return
						}

						err = m.InsertNewProfileBackground(backgroundName, backgroundUrl, tags)
						if err != nil {
							helpers.Relax(err)
						}
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

						if m.ProfileBackgroundNameExists(backgroundName) == false {
							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-delete-error-not-found"))
							return
						}
						backgroundUrl := m.GetProfileBackgroundUrl(backgroundName)

						if helpers.ConfirmEmbed(
							msg.ChannelID, msg.Author, helpers.GetTextF("plugins.levels.profile-background-delete-confirm",
								backgroundName, backgroundUrl),
							"✅", "🚫") == true {
							err = m.DeleteProfileBackground(backgroundName)
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

							_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-delete-success"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						}
						return
					})
					return
				default:
					if m.ProfileBackgroundNameExists(args[1]) == false {
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
					userUserdata.Background = args[1]
					_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
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
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
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

							imageUrl, err := helpers.UploadImage(badgeData)
							helpers.Relax(err)

							newBadge := new(DB_Badge)

							newBadge.CreatedByUserID = msg.Author.ID
							newBadge.GuildID = channel.GuildID
							newBadge.CreatedAt = time.Now()
							newBadge.Category = strings.ToLower(args[2])
							newBadge.Name = strings.ToLower(args[3])
							newBadge.URL = imageUrl                                      // reupload to imgur
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
							picData, err := helpers.NetGetUAWithError(newBadge.URL, helpers.DEFAULT_UA)
							if err != nil {
								if _, ok := err.(*url.Error); ok {
									_, err = helpers.SendMessage(msg.ChannelID, "Invalid url.")
									helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								} else {
									helpers.Relax(err)
								}
								return
							}
							newBadge.URL, err = helpers.UploadImage(picData)
							if err != nil {
								if strings.Contains(err.Error(), "Invalid URL") {
									_, err = helpers.SendMessage(msg.ChannelID, "I wasn't able to reupload the picture. Please make sure it is a direct link to the image.")
									helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								} else {
									helpers.Relax(err)
								}
								return
							}

							badgeFound := m.GetBadge(newBadge.Category, newBadge.Name, channel.GuildID)
							if badgeFound.ID != "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.create-badge-error-duplicate"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							serverBadges := m.GetServerOnlyBadges(channel.GuildID)
							badgeLimit := helpers.GuildSettingsGetCached(channel.GuildID).LevelsMaxBadges
							if badgeLimit == 0 {
								badgeLimit = 100
							}
							if len(serverBadges) >= badgeLimit {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.levels.create-badge-error-too-many", helpers.GetStaffUsernamesText()))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
								return
							}

							badgeID, err := m.InsertBadge(*newBadge)
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, badgeID,
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
										Value: newBadge.URL,
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
									},
									{
										Key:   "badge_guildid",
										Value: newBadge.GuildID,
									},
									{
										Key:   "badge_alloweduserids",
										Value: strings.Join(newBadge.AllowedUserIDs, ","),
									},
									{
										Key:   "badge_denieduserids",
										Value: strings.Join(newBadge.DeniedUserIDs, ","),
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

							badgeFound := m.GetBadge(args[2], args[3], channel.GuildID)
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

							m.DeleteBadge(badgeFound.ID)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, badgeFound.ID,
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
										Value: badgeFound.URL,
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
									},
									{
										Key:   "badge_guildid",
										Value: badgeFound.GuildID,
									},
									{
										Key:   "badge_alloweduserids",
										Value: strings.Join(badgeFound.AllowedUserIDs, ","),
									},
									{
										Key:   "badge_denieduserids",
										Value: strings.Join(badgeFound.DeniedUserIDs, ","),
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

							categoryBadges := m.GetCategoryBadges(categoryName, channel.GuildID)

							if len(categoryBadges) <= 0 {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.list-category-badge-error-none"))
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
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
									globalText, badge.Name, badge.URL, badge.BorderColor, requirementText, len(badge.AllowedUserIDs), len(badge.DeniedUserIDs),
								)
							}
							resultText += fmt.Sprintf("I found %d badges in this category.\n",
								len(categoryBadges))

							for _, page := range helpers.Pagify(resultText, "\n") {
								_, err = helpers.SendMessage(msg.ChannelID, page)
								helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							}
							return
						}

						channel, err := helpers.GetChannel(msg.ChannelID)
						helpers.Relax(err)

						serverBadges := m.GetServerBadges(channel.GuildID)

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

							badgeToAllow := m.GetBadge(args[3], args[4], channel.GuildID)
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
								m.UpdateBadge(badgeToAllow)

								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, badgeToAllow.ID,
									models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
									models.EventlogTypeRobyulBadgeAllow, "",
									[]models.ElasticEventlogChange{
										{
											Key:      "badge_alloweduserids",
											OldValue: strings.Join(allowedIDsBefore, ","),
											NewValue: strings.Join(badgeToAllow.AllowedUserIDs, ","),
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
											Value: badgeToAllow.URL,
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
										},
										{
											Key:   "badge_guildid",
											Value: badgeToAllow.GuildID,
										},
										{
											Key:   "badge_alloweduserids_added",
											Value: targetUser.ID,
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
								m.UpdateBadge(badgeToAllow)

								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, badgeToAllow.ID,
									models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
									models.EventlogTypeRobyulBadgeAllow, "",
									[]models.ElasticEventlogChange{
										{
											Key:      "badge_alloweduserids",
											OldValue: strings.Join(allowedIDsBefore, ","),
											NewValue: strings.Join(badgeToAllow.AllowedUserIDs, ","),
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
											Value: badgeToAllow.URL,
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
										},
										{
											Key:   "badge_guildid",
											Value: badgeToAllow.GuildID,
										},
										{
											Key:   "badge_alloweduserids_removed",
											Value: targetUser.ID,
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

							badgeToDeny := m.GetBadge(args[3], args[4], channel.GuildID)
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
								m.UpdateBadge(badgeToDeny)

								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, badgeToDeny.ID,
									models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
									models.EventlogTypeRobyulBadgeDeny, "",
									[]models.ElasticEventlogChange{
										{
											Key:      "badge_denieduserids",
											OldValue: strings.Join(deniedIDsBefore, ","),
											NewValue: strings.Join(badgeToDeny.DeniedUserIDs, ","),
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
											Value: badgeToDeny.URL,
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
										},
										{
											Key:   "badge_guildid",
											Value: badgeToDeny.GuildID,
										},
										{
											Key:   "badge_denieduserids_added",
											Value: targetUser.ID,
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
								m.UpdateBadge(badgeToDeny)

								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, badgeToDeny.ID,
									models.EventlogTargetTypeRobyulBadge, msg.Author.ID,
									models.EventlogTypeRobyulBadgeDeny, "",
									[]models.ElasticEventlogChange{
										{
											Key:      "badge_denieduserids",
											OldValue: strings.Join(deniedIDsBefore, ","),
											NewValue: strings.Join(badgeToDeny.DeniedUserIDs, ","),
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
											Value: badgeToDeny.URL,
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
										},
										{
											Key:   "badge_guildid",
											Value: badgeToDeny.GuildID,
										},
										{
											Key:   "badge_denieduserids_removed",
											Value: targetUser.ID,
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

						idToMove := ""
						for _, badgeID := range userData.ActiveBadgeIDs {
							badge := m.GetBadgeByID(badgeID)
							if badge.Category == categoryName && badge.Name == badgeName {
								idToMove = badge.ID
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
						_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
						helpers.Relax(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.move-badge-success"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

						return
					}
				}
				session.ChannelTyping(msg.ChannelID)

				availableBadges := m.GetBadgesAvailable(msg.Author, channel.GuildID)

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
						if availableBadge.ID == activeBadgeID {
							newActiveBadgeIDs = append(newActiveBadgeIDs, activeBadgeID)
						}
					}
				}
				userData.ActiveBadgeIDs = newActiveBadgeIDs

				shownBadges := make([]DB_Badge, 0)
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
									_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
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
													if activeBadgeID == badge.ID {
														newActiveBadges := make([]string, 0)
														for _, newActiveBadgeID := range userData.ActiveBadgeIDs {
															if newActiveBadgeID != badge.ID {
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
												userData.ActiveBadgeIDs = append(userData.ActiveBadgeIDs, badge.ID)
												if len(userData.ActiveBadgeIDs) >= BadgeLimt {
													_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
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
					_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userData.ID, userData)
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
				_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
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

				_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
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
				_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
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
				_, err = helpers.MDbUpdate(models.ProfileUserdataTable, userUserdata.ID, userUserdata)
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
		if err != nil && strings.Contains(err.Error(), "exit status 1") {
			cache.GetLogger().WithField("module", "levels").Error(fmt.Sprintf("Profile generation failed: %#v", err))
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.levels.profile-error-exit1"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}
		helpers.Relax(err)

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
					//fmt.Println("displayRanking:", displayRanking, "i:", i, "offset:", offset)
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

					currentMember, err := helpers.GetGuildMemberWithoutApi(channel.GuildID, levelsServersUsers[i-offset].UserID)
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
						Value:  fmt.Sprintf("Level: %d", m.getLevelFromExp(levelsServersUsers[i-offset].Exp)),
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
						Value:  fmt.Sprintf("Level: %d", m.getLevelFromExp(thislevelUser.Exp)),
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
						Value:  fmt.Sprintf("Global Level: %d", m.getLevelFromExp(userRanked.Value)),
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
						Value:  fmt.Sprintf("Global Level: %d", m.getLevelFromExp(totalExp)),
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
							_, err = helpers.MDbUpdate(models.LevelsServerusersTable, levelsServerUser.ID, levelsServerUser)

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
									ignoredUser, err := helpers.GetGuildMember(channel.GuildID, ignoredUserID)
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
											},
										},
										[]models.ElasticEventlogOption{
											{
												Key:   "levels_ignoreduserids_removed",
												Value: targetUser.ID,
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
									},
								},
								[]models.ElasticEventlogOption{
									{
										Key:   "levels_ignoreduserids_added",
										Value: targetUser.ID,
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
											},
										},
										[]models.ElasticEventlogOption{
											{
												Key:   "levels_ignoredchannelids_removed",
												Value: targetChannel.ID,
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
									},
								},
								[]models.ElasticEventlogOption{
									{
										Key:   "levels_ignoredchannelids_added",
										Value: targetChannel.ID,
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
							_, err = helpers.MDbUpdate(models.LevelsServerusersTable, levelsServerUser.ID, levelsServerUser)
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
							_, err = helpers.MDbUpdate(models.LevelsServerusersTable, levelsServerUser.ID, levelsServerUser)
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

								errRole := m.applyLevelsRoles(guild.ID, member.User.ID, m.GetLevelForUser(member.User.ID, guild.ID))
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

							err = m.applyLevelsRoles(guild.ID, targetUser.ID, m.GetLevelForUser(targetUser.ID, guild.ID))
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
								models.EventlogTargetTypeUser, msg.Author.ID,
								models.EventlogTypeRobyulLevelsRoleGrant, "",
								nil,
								[]models.ElasticEventlogOption{
									{
										Key:   "levels_role_grant_roleid_removed",
										Value: targetRole.ID,
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

						err = m.applyLevelsRoles(guild.ID, targetUser.ID, m.GetLevelForUser(targetUser.ID, guild.ID))
						helpers.Relax(err)

						_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
							models.EventlogTargetTypeUser, msg.Author.ID,
							models.EventlogTypeRobyulLevelsRoleGrant, "",
							nil,
							[]models.ElasticEventlogOption{
								{
									Key:   "levels_role_grant_roleid_added",
									Value: targetRole.ID,
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

							err = m.applyLevelsRoles(guild.ID, targetUser.ID, m.GetLevelForUser(targetUser.ID, guild.ID))
							helpers.Relax(err)

							_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
								models.EventlogTargetTypeUser, msg.Author.ID,
								models.EventlogTypeRobyulLevelsRoleDeny, "",
								nil,
								[]models.ElasticEventlogOption{
									{
										Key:   "levels_role_deny_roleid_removed",
										Value: targetRole.ID,
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

						err = m.applyLevelsRoles(guild.ID, targetUser.ID, m.GetLevelForUser(targetUser.ID, guild.ID))
						helpers.Relax(err)

						_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
							models.EventlogTargetTypeUser, msg.Author.ID,
							models.EventlogTypeRobyulLevelsRoleDeny, "",
							nil,
							[]models.ElasticEventlogOption{
								{
									Key:   "levels_role_deny_roleid_added",
									Value: targetRole.ID,
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
					Value:  strconv.Itoa(m.getLevelFromExp(levelThisServerUser.Exp)),
					Inline: true,
				},
				{
					Name:   "Level Progress",
					Value:  strconv.Itoa(m.getProgressToNextLevelFromExp(levelThisServerUser.Exp)) + " %",
					Inline: true,
				},
				{
					Name:   zeroWidthWhitespace,
					Value:  zeroWidthWhitespace,
					Inline: true,
				},
				{
					Name:   "Global Level",
					Value:  strconv.Itoa(m.getLevelFromExp(totalExp)),
					Inline: true,
				},
				{
					Name:   "Global Level Progress",
					Value:  strconv.Itoa(m.getProgressToNextLevelFromExp(totalExp)) + " %",
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

func (l *Levels) BadgePickerPrintCategories(user *discordgo.User, channeID string, availableBadges []DB_Badge, activeBadgeIDs []string, allBadges []DB_Badge) []string {
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

func (l *Levels) BadgePickerPrintBadges(user *discordgo.User, channeID string, availableBadges []DB_Badge, activeBadgeIDs []string, categoryName string, allBadges []DB_Badge) []string {
	categoryName = strings.ToLower(categoryName)

	resultText := l.BadgePickerActiveText(user.Username, activeBadgeIDs, allBadges)
	resultText += "Choose a badge name:\n"
	for _, badge := range availableBadges {
		if badge.Category == categoryName {
			isActive := false
			for _, activeBadgeID := range activeBadgeIDs {
				if activeBadgeID == badge.ID {
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

func (l *Levels) BadgePickerActiveText(username string, activeBadgeIDs []string, availableBadges []DB_Badge) string {
	spaceLeft := BadgeLimt - len(activeBadgeIDs)
	text := fmt.Sprintf("**@%s** You can pick %d more badge(s) to display on your profile.\nYou are currently displaying:", username, spaceLeft)
	if len(activeBadgeIDs) > 0 {
		for _, badgeID := range activeBadgeIDs {
			for _, badge := range availableBadges {
				if badge.ID == badgeID {
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

func (l *Levels) InsertNewProfileBackground(backgroundName string, backgroundUrl string, tags []string) error {
	newEntry := new(DB_Profile_Background)
	newEntry.Name = strings.ToLower(backgroundName)
	newEntry.URL = backgroundUrl
	newEntry.CreatedAt = time.Now()
	newEntry.Tags = tags

	insert := rethink.Table("profile_backgrounds").Insert(newEntry)
	_, err := insert.RunWrite(helpers.GetDB())
	return err
}

func (l *Levels) DeleteProfileBackground(backgroundName string) error {
	if backgroundName != "" {
		_, err := rethink.Table("profile_backgrounds").Filter(
			rethink.Row.Field("id").Eq(backgroundName),
		).Delete().RunWrite(helpers.GetDB())
		return err
	}
	return nil
}

func (l *Levels) ProfileBackgroundSearch(searchText string) []DB_Profile_Background {
	var entryBucket []DB_Profile_Background
	listCursor, err := rethink.Table("profile_backgrounds").Filter(func(profile rethink.Term) rethink.Term {
		return profile.Field("id").Match(fmt.Sprintf("(?i)%s", searchText))
	}).Limit(5).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return entryBucket
	} else if err != nil {
		helpers.Relax(err)
	}

	return entryBucket
}

func (l *Levels) ProfileBackgroundNameExists(backgroundName string) bool {
	var entryBucket DB_Profile_Background
	listCursor, err := rethink.Table("profile_backgrounds").Filter(
		rethink.Row.Field("id").Eq(strings.ToLower(backgroundName)),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		var entryBucket DB_Profile_Background
		listCursor, err := rethink.Table("profile_backgrounds").Filter(func(profile rethink.Term) rethink.Term {
			return profile.Field("id").Match(fmt.Sprintf("(?i)^%s$", regexp.QuoteMeta(backgroundName)))
		}).Run(helpers.GetDB())
		if err != nil {
			panic(err)
		}
		defer listCursor.Close()
		err = listCursor.One(&entryBucket)

		if err == rethink.ErrEmptyResult {
			return false
		} else if err != nil {
			helpers.Relax(err)
		}

		return true
	} else if err != nil {
		helpers.Relax(err)
	}

	return true
}

func (l *Levels) GetProfileBackgroundUrl(backgroundName string) string {
	if backgroundName == "" {
		return "http://i.imgur.com/I9b74U9.jpg"
	}

	var entryBucket DB_Profile_Background
	listCursor, err := rethink.Table("profile_backgrounds").Filter(
		rethink.Row.Field("id").Eq(strings.ToLower(backgroundName)),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		var entryBucket DB_Profile_Background
		listCursor, err := rethink.Table("profile_backgrounds").Filter(func(profile rethink.Term) rethink.Term {
			return profile.Field("id").Match(fmt.Sprintf("(?i)^%s$", regexp.QuoteMeta(backgroundName)))
		}).Run(helpers.GetDB())
		if err != nil {
			panic(err)
		}
		defer listCursor.Close()
		err = listCursor.One(&entryBucket)

		if err == rethink.ErrEmptyResult {
			if strings.HasPrefix(backgroundName, "http") {
				return backgroundName
			}

			return "http://i.imgur.com/I9b74U9.jpg" // Default Robyul Background
		} else if err != nil {
			helpers.Relax(err)
		}

		return entryBucket.URL
	} else if err != nil {
		helpers.Relax(err)
	}

	return entryBucket.URL
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

func (l *Levels) GetServerOnlyBadges(guildID string) []DB_Badge {
	var entryBucket []DB_Badge
	listCursor, err := rethink.Table("profile_badge").Filter(
		rethink.Row.Field("guildid").Eq(guildID),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err != nil {
		if err != rethink.ErrEmptyResult {
			helpers.Relax(err)
		}
	}

	return entryBucket
}

func (l *Levels) GetServerBadges(guildID string) []DB_Badge {
	var entryBucket []DB_Badge
	var globalEntryBucket []DB_Badge
	listCursor, err := rethink.Table("profile_badge").Filter(
		rethink.Row.Field("guildid").Eq(guildID),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)
	if err != nil {
		panic(err)
	}

	listCursor, err = rethink.Table("profile_badge").Filter(
		rethink.Row.Field("guildid").Eq("global"),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.All(&globalEntryBucket)

	for _, globalEntry := range globalEntryBucket {
		entryBucket = append(entryBucket, globalEntry)
	}

	if err != nil {
		if err != rethink.ErrEmptyResult {
			helpers.Relax(err)
		}
	}

	return entryBucket
}

func (l *Levels) GetCategoryBadges(category string, guildID string) []DB_Badge {
	var entryBucket []DB_Badge
	result := make([]DB_Badge, 0)
	listCursor, err := rethink.Table("profile_badge").Filter(
		rethink.Row.Field("category").Eq(category),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return result
	} else if err != nil {
		helpers.Relax(err)
	}

	for _, badge := range entryBucket {
		if badge.GuildID == guildID || badge.GuildID == "global" {
			result = append(result, badge)
		}
	}

	return result
}

func (l *Levels) GetBadge(category string, name string, guildID string) DB_Badge {
	var entryBucket []DB_Badge
	var emptyBadge DB_Badge
	listCursor, err := rethink.Table("profile_badge").Filter(
		rethink.Row.Field("category").Eq(strings.ToLower(category)),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return emptyBadge
	} else if err != nil {
		helpers.Relax(err)
	}

	for _, badge := range entryBucket {
		if strings.ToLower(badge.Name) == strings.ToLower(name) && (badge.GuildID == guildID || badge.GuildID == "global") {
			return badge
		}
	}

	return emptyBadge
}

func (l *Levels) GetBadgeByID(badgeID string) DB_Badge {
	var badgeBucket DB_Badge
	listCursor, err := rethink.Table("profile_badge").Filter(
		rethink.Row.Field("id").Eq(badgeID),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&badgeBucket)

	if err == rethink.ErrEmptyResult {
		return badgeBucket
	} else if err != nil {
		panic(err)
	}

	return badgeBucket
}

func (l *Levels) GetBadgesAvailable(user *discordgo.User, sourceServerID string) []DB_Badge {
	guildsToCheck := make([]string, 0)
	guildsToCheck = append(guildsToCheck, "global")

	session := cache.GetSession()

	for _, guild := range session.State.Guilds {
		if helpers.GetIsInGuild(guild.ID, user.ID) {
			guildsToCheck = append(guildsToCheck, guild.ID)
		}
	}

	sourceServerAlreadyIn := false
	for _, guild := range guildsToCheck {
		if guild == sourceServerID {
			sourceServerAlreadyIn = true
		}
	}
	if sourceServerAlreadyIn == false {
		guildsToCheck = append(guildsToCheck, sourceServerID)
	}

	var allBadges []DB_Badge
	for _, guildToCheck := range guildsToCheck {
		var entryBucket []DB_Badge
		listCursor, err := rethink.Table("profile_badge").Filter(
			rethink.Row.Field("guildid").Eq(guildToCheck),
		).Run(helpers.GetDB())
		defer listCursor.Close()
		if err != nil {
			continue
		}
		err = listCursor.All(&entryBucket)
		if err != nil {
			continue
		}
		for _, entryBadge := range entryBucket {
			allBadges = append(allBadges, entryBadge)
		}
	}

	levelCache := make(map[string]int, 0)

	var availableBadges []DB_Badge
	for _, foundBadge := range allBadges {
		isAllowed := false

		// Level Check
		if foundBadge.LevelRequirement < 0 { // Available for no one?
			isAllowed = false
		} else if foundBadge.LevelRequirement == 0 { // Available for everyone?
			isAllowed = true
		} else if foundBadge.LevelRequirement > 0 { // Meets min level?
			if _, ok := levelCache[foundBadge.GuildID]; ok {
				if foundBadge.LevelRequirement <= levelCache[foundBadge.GuildID] {
					isAllowed = true
				} else {
					isAllowed = false
				}
			} else {
				levelCache[foundBadge.GuildID] = l.GetLevelForUser(user.ID, foundBadge.GuildID)
				if foundBadge.LevelRequirement <= levelCache[foundBadge.GuildID] {
					isAllowed = true
				} else {
					isAllowed = false
				}
			}
		}
		// Role Check
		if foundBadge.RoleRequirement != "" { // is there a role requirement?
			isAllowed = false
			member, err := helpers.GetGuildMember(foundBadge.GuildID, user.ID)
			if err == nil {
				for _, memberRole := range member.Roles { // check if user got role
					if memberRole == foundBadge.RoleRequirement {
						isAllowed = true
					}
				}
			}
		}

		// User is in allowed user list?
		for _, allowedUserID := range foundBadge.AllowedUserIDs {
			if allowedUserID == user.ID {
				isAllowed = true
			}
		}

		// User is in denied user list?
		for _, deniedUserID := range foundBadge.DeniedUserIDs {
			if deniedUserID == user.ID {
				isAllowed = false
			}
		}

		if isAllowed == true {
			availableBadges = append(availableBadges, foundBadge)
		}
	}

	return availableBadges
}

func (l *Levels) GetBadgesAvailableQuick(user *discordgo.User, activeBadgeIDs []string) []DB_Badge {
	activeBadges := make([]DB_Badge, 0)
	for _, activeBadgeID := range activeBadgeIDs {
		badge := l.GetBadgeByID(activeBadgeID)
		if badge.ID != "" {
			if badge.GuildID == "global" {
				activeBadges = append(activeBadges, badge)
			} else {
				if helpers.GetIsInGuild(badge.GuildID, user.ID) {
					activeBadges = append(activeBadges, badge)
				}
			}
		}
	}

	var availableBadges []DB_Badge
	for _, foundBadge := range activeBadges {
		isAllowed := true

		// User is in allowed user list?
		for _, allowedUserID := range foundBadge.AllowedUserIDs {
			if allowedUserID == user.ID {
				isAllowed = true
			}
		}

		// User is in denied user list?
		for _, deniedUserID := range foundBadge.DeniedUserIDs {
			if deniedUserID == user.ID {
				isAllowed = false
			}
		}

		if isAllowed == true {
			availableBadges = append(availableBadges, foundBadge)
		}
	}

	return availableBadges
}

func (l *Levels) GetLevelForUser(userID string, guildID string) int {
	var levelsServersUser []models.LevelsServerusersEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"userid": userID})).All(&levelsServersUser)
	helpers.Relax(err)

	if levelsServersUser == nil {
		return 0
	} else if err != nil {
		helpers.Relax(err)
	}

	if guildID == "global" {
		totalExp := int64(0)
		for _, levelsServerUser := range levelsServersUser {
			totalExp += levelsServerUser.Exp
		}
		return l.getLevelFromExp(totalExp)
	} else {
		for _, levelsServerUser := range levelsServersUser {
			if levelsServerUser.GuildID == guildID {
				return l.getLevelFromExp(levelsServerUser.Exp)
			}
		}
	}

	return 0
}

func (l *Levels) InsertBadge(entry DB_Badge) (badgeID string, err error) {
	insert := rethink.Table("profile_badge").Insert(entry)
	res, err := insert.RunWrite(helpers.GetDB())
	if err != nil {
		return "", err
	}
	return res.GeneratedKeys[0], nil
}

func (l *Levels) UpdateBadge(entry DB_Badge) {
	if entry.ID != "" {
		_, err := rethink.Table("profile_badge").Update(entry).Run(helpers.GetDB())
		helpers.Relax(err)
	}
}

func (l *Levels) DeleteBadge(badgeID string) {
	_, err := rethink.Table("profile_badge").Filter(
		rethink.Row.Field("id").Eq(badgeID),
	).Delete().RunWrite(helpers.GetDB())
	helpers.Relax(err)
	return
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
	if len(userWithDisc) >= 15 {
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

	badgesToDisplay := make([]DB_Badge, 0)
	availableBadges := m.GetBadgesAvailableQuick(member.User, userData.ActiveBadgeIDs)
	for _, activeBadgeID := range userData.ActiveBadgeIDs {
		for _, availableBadge := range availableBadges {
			if activeBadgeID == availableBadge.ID {
				badgesToDisplay = append(badgesToDisplay, availableBadge)
			}
		}
	}
	var badgesHTML1, badgesHTML2 string
	for i, badge := range badgesToDisplay {
		if i <= 8 {
			badgesHTML1 += fmt.Sprintf("<img src=\"%s\" style=\"border: 2px solid #%s;\">", badge.URL, badge.BorderColor)
		} else {
			badgesHTML2 += fmt.Sprintf("<img src=\"%s\" style=\"border: 2px solid #%s;\">", badge.URL, badge.BorderColor)
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

	expOpacity := m.GetExpOpacity(userData)
	badgeOpacity := m.GetBadgeOpacity(userData)

	tempTemplateHtml := strings.Replace(htmlTemplateString, "{USER_USERNAME}", html.EscapeString(member.User.Username), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_NICKNAME}", html.EscapeString(member.Nick), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_AND_NICKNAME}", html.EscapeString(userAndNick), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USERNAME_WITH_DISC}", html.EscapeString(userWithDisc), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_AVATAR_URL}", html.EscapeString(avatarUrl), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TITLE}", html.EscapeString(title), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BIO}", html.EscapeString(bio), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_LEVEL}", strconv.Itoa(m.getLevelFromExp(levelThisServerUser.Exp)), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_RANK}", serverRank, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_LEVEL_PERCENT}", strconv.Itoa(m.getProgressToNextLevelFromExp(levelThisServerUser.Exp)), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_GLOBAL_LEVEL}", strconv.Itoa(m.getLevelFromExp(totalExp)), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_GLOBAL_RANK}", globalRank, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BACKGROUND_URL}", m.GetProfileBackgroundUrl(userData.Background), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_REP}", strconv.Itoa(userData.Rep), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BADGES_HTML_1}", badgesHTML1, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BADGES_HTML_2}", badgesHTML2, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BACKGROUND_COLOR}", html.EscapeString(backgroundColorString), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_ACCENT_COLOR}", "#"+m.GetAccentColor(userData), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_DETAIL_COLOR}", html.EscapeString(detailColorString), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TEXT_COLOR}", "#"+m.GetTextColor(userData), -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_EXP_OPACITY}", expOpacity, -1)
	tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BADGE_OPACITY}", badgeOpacity, -1)

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

	tempTemplatePath := cachePath + strconv.FormatInt(time.Now().UnixNano(), 10) + member.User.ID + ".html"
	err = ioutil.WriteFile(tempTemplatePath, []byte(tempTemplateHtml), 0644)
	if err != nil {
		return []byte{}, "", err
	}

	start := time.Now()

	cmdArgs := []string{
		tempTemplatePath,
		"--window-size=400/300",
		"--stream-type=png",
		//"--timeout=20000",
		"--p:disk-cache=true",
		"--p:disk-cache-path=" + cachePath,
		"--p:proxy-type=none",
		"--p:ignore-ssl-errors=false",
		"--p:ssl-protocol=any",
		"--p:web-security=false",
		"--p:debug=true",
	}
	// fmt.Println(webshotBinary, strings.Join(cmdArgs, " "))
	imgCmd := exec.Command(webshotBinary, cmdArgs...)
	imgCmd.Env = levelsEnv
	imageBytes, err := imgCmd.Output()
	if err != nil {
		return []byte{}, "", err
	}

	elapsed := time.Since(start)
	cache.GetLogger().WithField("module", "levels").Info(fmt.Sprintf("took screenshot of profile in %s", elapsed.String()))

	err = os.Remove(tempTemplatePath)
	if err != nil {
		return []byte{}, "", err
	}

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

	expStack.Push(ProcessExpInfo{UserID: msg.Author.ID, GuildID: channel.GuildID})
}

func (m *Levels) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		if member.User == nil {
			return
		}

		err := m.applyLevelsRoles(member.GuildID, member.User.ID, m.GetLevelForUser(member.User.ID, member.GuildID))
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

func (m *Levels) getLevelsServerUserOrCreateNewWithoutLogging(guildid string, userid string) (serveruser models.LevelsServerusersEntry, err error) {
	err = helpers.MdbOneWithoutLogging(
		helpers.MdbCollection(models.LevelsServerusersTable).Find(bson.M{"userid": userid, "guildid": guildid}),
		&serveruser,
	)

	if err == mgo.ErrNotFound {
		serveruser.UserID = userid
		serveruser.GuildID = guildid
		newid, err := helpers.MDbInsertWithoutLogging(models.LevelsServerusersTable, serveruser)
		serveruser.ID = newid
		return serveruser, err
	}

	return serveruser, err
}

func (m *Levels) getLevelFromExp(exp int64) int {
	calculatedLevel := 0.1 * math.Sqrt(float64(exp))

	return int(math.Floor(calculatedLevel))
}

func (m *Levels) getExpForLevel(level int) int64 {
	if level <= 0 {
		return 0
	}

	calculatedExp := math.Pow(float64(level)/0.1, 2)
	return int64(calculatedExp)
}

func (m *Levels) getProgressToNextLevelFromExp(exp int64) int {
	expLevelCurrently := exp - m.getExpForLevel(m.getLevelFromExp(exp))
	expLevelNext := m.getExpForLevel(m.getLevelFromExp(exp)+1) - m.getExpForLevel(m.getLevelFromExp(exp))
	return int(expLevelCurrently / (expLevelNext / 100))
}

func (m *Levels) getRandomExpForMessage() int64 {
	min := 10
	max := 15
	rand.Seed(time.Now().Unix())
	return int64(rand.Intn(max-min) + min)
}

func (m *Levels) rankMapByExp(exp map[string]int64) PairList {
	pl := make(PairList, len(exp))
	i := 0
	for k, v := range exp {
		pl[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(pl))
	return pl
}

type Pair struct {
	Key   string
	Value int64
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (b *Levels) BucketInit() {
	b.Lock()
	b.buckets = make(map[string]int8)
	b.Unlock()

	go b.BucketRefiller()
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

func (l *Levels) getLevelsRoles(guildID string, currentLevel int) (apply []*discordgo.Role, remove []*discordgo.Role) {
	apply = make([]*discordgo.Role, 0)
	remove = make([]*discordgo.Role, 0)

	var entryBucket []models.LevelsRoleEntry
	listCursor, err := rethink.Table(models.LevelsRolesTable).Filter(
		rethink.Row.Field("guild_id").Eq(guildID),
	).Run(helpers.GetDB())
	if err != nil {
		helpers.RelaxLog(err)
		return
	}
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return
	}

	for _, entry := range entryBucket {
		role, err := cache.GetSession().State.Role(guildID, entry.RoleID)
		if err != nil {
			continue
		}

		if currentLevel >= entry.StartLevel && (entry.LastLevel < 0 || currentLevel <= entry.LastLevel) {
			apply = append(apply, role)
		} else {
			remove = append(remove, role)
		}
	}

	return
}

func (l *Levels) applyLevelsRoles(guildID string, userID string, level int) (err error) {
	apply, remove := l.getLevelsRoles(guildID, level)
	member, err := helpers.GetGuildMember(guildID, userID)
	if err != nil {
		cache.GetLogger().WithField("module", "levels").Warnf("failed to get guild member to apply level roles: %s", err.Error())
		return err
	}

	toRemove := make([]*discordgo.Role, 0)
	toApply := make([]*discordgo.Role, 0)

	for _, removeRole := range remove {
		for _, memberRole := range member.Roles {
			if removeRole.ID == memberRole {
				toRemove = append(toRemove, removeRole)
			}
		}
	}
	for _, applyRole := range apply {
		hasRoleAlready := false
		for _, memberRole := range member.Roles {
			if applyRole.ID == memberRole {
				hasRoleAlready = true
			}
		}
		if !hasRoleAlready {
			toApply = append(toApply, applyRole)
		}
	}

	session := cache.GetSession()

	overwrites := l.getLevelsRolesUserOverwrites(guildID, userID)
	for _, overwrite := range overwrites {
		switch overwrite.Type {
		case "grant":
			hasRoleAlready := false
			for _, memberRole := range member.Roles {
				if overwrite.RoleID == memberRole {
					hasRoleAlready = true
				}
			}
			if !hasRoleAlready {
				applyingAlready := false
				for _, applyingRole := range toApply {
					if applyingRole.ID == overwrite.RoleID {
						applyingAlready = true
					}
				}

				if !applyingAlready {
					applyRole, err := session.State.Role(guildID, overwrite.RoleID)

					if err == nil {
						toApply = append(toApply, applyRole)
					}
				}
			}

			newToRemove := make([]*discordgo.Role, 0)
			for _, role := range toRemove {
				if role.ID != overwrite.RoleID {
					newToRemove = append(newToRemove, role)
				}
			}
			toRemove = newToRemove

			break
		case "deny":
			hasRole := false
			for _, memberRole := range member.Roles {
				if overwrite.RoleID == memberRole {
					hasRole = true
				}
			}

			if hasRole {
				removeRole, err := session.State.Role(guildID, overwrite.RoleID)
				if err == nil {
					toRemove = append(toRemove, removeRole)
				}
			}

			newToApply := make([]*discordgo.Role, 0)
			for _, role := range toApply {
				if role.ID != overwrite.RoleID {
					newToApply = append(newToApply, role)
				}
			}
			toApply = newToApply

			break
		}
	}

	for _, toApplyRole := range toApply {
		errRole := session.GuildMemberRoleAdd(guildID, userID, toApplyRole.ID)
		if errRole != nil {
			cache.GetLogger().WithField("module", "levels").Warnf("failed to add role applying level roles: %s", errRole.Error())
			err = errRole
		}
	}

	for _, toRemoveRole := range toRemove {
		errRole := session.GuildMemberRoleRemove(guildID, userID, toRemoveRole.ID)
		if errRole != nil {
			cache.GetLogger().WithField("module", "levels").Warnf("failed to remove role applying level roles: %s", errRole.Error())
			err = errRole
		}
	}

	return
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

func (l *Levels) getLevelsRolesUserOverwrites(guildID string, userID string) (overwrites []models.LevelsRoleOverwriteEntry) {
	listCursor, err := rethink.Table(models.LevelsRoleOverwritesTable).Filter(
		rethink.And(
			rethink.Row.Field("guild_id").Eq(guildID),
			rethink.Row.Field("user_id").Eq(userID),
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
