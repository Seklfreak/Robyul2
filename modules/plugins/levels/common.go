package levels

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	raven "github.com/getsentry/raven-go"
	redisCache "github.com/go-redis/cache"
)

func setServerFeaturesLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "levels").Error("The setServerFeaturesLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			setServerFeaturesLoop()
		}()
	}()

	var badgesBucket []models.ProfileBadgeEntry
	var badgesOnServer []models.ProfileBadgeEntry
	var err error
	var feature models.Rest_Feature_Levels_Badges
	var key string
	cacheCodec := cache.GetRedisCacheCodec()
	for {
		err = helpers.MDbIter(helpers.MdbCollection(models.ProfileBadgesTable).Find(nil)).All(&badgesBucket)
		if err != nil {
			helpers.RelaxLog(err)
			time.Sleep(60 * time.Second)
			continue
		}

		for _, shard := range cache.GetSession().Sessions {
			for _, guild := range shard.State.Guilds {
				badgesOnServer = make([]models.ProfileBadgeEntry, 0)
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
		}

		time.Sleep(30 * time.Minute)
	}
}

func cacheTopLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "levels").Error("The cacheTopLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			cacheTopLoop()
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

		for _, shard := range cache.GetSession().Sessions {
			for _, guild := range shard.State.Guilds {
				guildExpMap := make(map[string]int64, 0)
				for _, levelsUser := range levelsUsers {
					if levelsUser.GuildID == guild.ID {
						guildExpMap[levelsUser.UserID] = levelsUser.Exp
					}
				}
				rankedGuildExpMap := rankMapByExp(guildExpMap)
				newTopCache = append(newTopCache, Cache_Levels_top{
					GuildID: guild.ID,
					Levels:  rankedGuildExpMap,
				})
			}
		}

		totalExpMap := make(map[string]int64, 0)
		for _, levelsUser := range levelsUsers {
			if _, ok := totalExpMap[levelsUser.UserID]; ok {
				totalExpMap[levelsUser.UserID] += levelsUser.Exp
			} else {
				totalExpMap[levelsUser.UserID] = levelsUser.Exp
			}
		}

		rankedTotalExpMap := rankMapByExp(totalExpMap)
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
						Level:   GetLevelFromExp(level.Value),
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

		newTopCache = nil
		levelsUsers = nil

		time.Sleep(10 * time.Minute)
	}
}

func processExpStackLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "levels").Info("The processExpStackLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			processExpStackLoop()
		}()
	}()

	for {
		metrics.LevelsStackSize.Set(int64(expStack.Size()))
		if !expStack.Empty() {
			expItem := expStack.Pop().(ProcessExpInfo)
			levelsServerUser, err := getLevelsServerUserOrCreateNewWithoutLogging(expItem.GuildID, expItem.UserID)
			helpers.Relax(err)

			expBefore := levelsServerUser.Exp
			levelBefore := GetLevelFromExp(levelsServerUser.Exp)

			levelsServerUser.Exp += getRandomExpForMessage()

			levelAfter := GetLevelFromExp(levelsServerUser.Exp)

			err = helpers.MDbUpdateWithoutLogging(models.LevelsServerusersTable, levelsServerUser.ID, levelsServerUser)
			helpers.Relax(err)

			if expBefore <= 0 || levelBefore != levelAfter {
				// apply roles
				err := applyLevelsRoles(expItem.GuildID, expItem.UserID, levelAfter)
				if errD, ok := err.(*discordgo.RESTError); !ok || (errD.Message.Message != "404: Not Found" &&
					errD.Message.Code != discordgo.ErrCodeUnknownMember &&
					errD.Message.Code != discordgo.ErrCodeMissingAccess) {
					helpers.RelaxLog(err)
				}
				guildSettings := helpers.GuildSettingsGetCached(expItem.GuildID)
				// send level notifications
				if levelAfter > levelBefore && guildSettings.LevelsNotificationCode != "" {
					go func() {
						defer helpers.Recover()

						member, err := helpers.GetGuildMemberWithoutApi(expItem.GuildID, expItem.UserID)
						helpers.RelaxLog(err)
						if err == nil {
							levelNotificationText := replaceLevelNotificationText(guildSettings.LevelsNotificationCode, member, levelAfter)
							if levelNotificationText == "" {
								return
							}
							messageSend := &discordgo.MessageSend{
								Content: levelNotificationText,
							}
							if helpers.IsEmbedCode(levelNotificationText) {
								ptext, embed, err := helpers.ParseEmbedCode(levelNotificationText)
								if err == nil {
									messageSend.Content = ptext
									messageSend.Embed = embed
								}
							}
							messages, err := helpers.SendComplex(expItem.ChannelID, messageSend)
							if err != nil {
								if errD, ok := err.(*discordgo.RESTError); ok {
									if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
										return
									}
								}
								helpers.RelaxLog(err)
								return
							}
							if messages != nil && guildSettings.LevelsNotificationDeleteAfter > 0 {
								go func() {
									defer helpers.Recover()

									time.Sleep(time.Duration(guildSettings.LevelsNotificationDeleteAfter) * time.Second)

									for _, message := range messages {
										cache.GetSession().SessionForGuildS(message.GuildID).ChannelMessageDelete(message.ChannelID, message.ID)
									}
								}()
							}
						}
						return
					}()
				}
			}
		} else {
			time.Sleep(250 * time.Millisecond)
		}
	}
}

func replaceLevelNotificationText(text string, member *discordgo.Member, newLevel int) string {
	text = strings.Replace(text, "{USER_USERNAME}", member.User.Username, -1)
	text = strings.Replace(text, "{USER_ID}", member.User.ID, -1)
	text = strings.Replace(text, "{USER_DISCRIMINATOR}", member.User.Discriminator, -1)
	text = strings.Replace(text, "{USER_MENTION}", fmt.Sprintf("<@%s>", member.User.ID), -1)
	text = strings.Replace(text, "{USER_AVATARURL}", member.User.AvatarURL(""), -1)
	text = strings.Replace(text, "{USER_NEWLEVEL}", strconv.Itoa(newLevel), -1)

	guild, err := helpers.GetGuild(member.GuildID)
	helpers.RelaxLog(err)
	if err == nil {
		text = strings.Replace(text, "{GUILD_NAME}", guild.Name, -1)
		text = strings.Replace(text, "{GUILD_ID}", guild.ID, -1)
	}

	return text
}

func rankMapByExp(exp map[string]int64) PairList {
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

func applyLevelsRoles(guildID string, userID string, level int) (err error) {
	apply, remove := getLevelsRoles(guildID, level)
	member, err := helpers.GetGuildMemberWithoutApi(guildID, userID)
	if err != nil {
		// cache.GetLogger().WithField("module", "levels").Warnf("failed to get guild member to apply level roles: %s", err.Error())
		return nil
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

	overwrites := getLevelsRolesUserOverwrites(guildID, userID)
	for _, overwrite := range overwrites {
		switch overwrite.Type {
		case models.LevelsRoleOverwriteTypeGrant:
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
					applyRole, err := session.SessionForGuildS(guildID).State.Role(guildID, overwrite.RoleID)

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
		case models.LevelsRoleOverwriteTypeDeny:
			hasRole := false
			for _, memberRole := range member.Roles {
				if overwrite.RoleID == memberRole {
					hasRole = true
				}
			}

			if hasRole {
				removeRole, err := session.SessionForGuildS(guildID).State.Role(guildID, overwrite.RoleID)
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
		errRole := session.SessionForGuildS(guildID).GuildMemberRoleAdd(guildID, userID, toApplyRole.ID)
		if errRole != nil {
			cache.GetLogger().WithField("module", "levels").Warnf("failed to add role applying level roles: %s", errRole.Error())
			err = errRole
		}
	}

	for _, toRemoveRole := range toRemove {
		errRole := session.SessionForGuildS(guildID).GuildMemberRoleRemove(guildID, userID, toRemoveRole.ID)
		if errRole != nil {
			cache.GetLogger().WithField("module", "levels").Warnf("failed to remove role applying level roles: %s", errRole.Error())
			err = errRole
		}
	}

	return
}
