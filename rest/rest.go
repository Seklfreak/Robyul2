package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/generator"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/modules/plugins"
	"github.com/bwmarrin/discordgo"
	"github.com/emicklei/go-restful"
	"github.com/getsentry/raven-go"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"
)

func NewRestServices() []*restful.WebService {
	services := make([]*restful.WebService, 0)

	service := new(restful.WebService)
	service.
		Path("/bot/guilds").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	service.Route(service.GET("").To(GetAllBotGuilds))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/user").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{user-id}").To(FindUser))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/member").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}/{user-id}").To(FindMember))
	service.Route(service.GET("/{guild-id}/{user-id}/is").To(IsMember))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/profile").
		Consumes(restful.MIME_JSON).
		Produces("text/html")

	service.Route(service.GET("/{user-id}/{guild-id}").To(GetProfile))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/rankings").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}").To(GetRankings))
	service.Route(service.GET("/user/{user-id}/{guild-id}").To(GetUserRanking))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/guild").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}").To(FindGuild))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/randompictures").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/history/{guild-id}/{start}/{end}").To(GetRandomPicturesGuildHistory))
	services = append(services, service)
	return services
}

func GetAllBotGuilds(request *restful.Request, response *restful.Response) {
	allGuilds := cache.GetSession().State.Guilds
	cacheCodec := cache.GetRedisCacheCodec()
	var key string
	var featureLevels_Badges models.Rest_Feature_Levels_Badges
	var featureRandomPictures models.Rest_Feature_RandomPictures
	var botPrefix string
	var err error

	returnGuilds := make([]models.Rest_Guild, 0)
	for _, guild := range allGuilds {
		joinedAt := helpers.GetTimeFromSnowflake(guild.ID)
		key = fmt.Sprintf(models.Redis_Key_Feature_Levels_Badges, guild.ID)
		if err = cacheCodec.Get(key, &featureLevels_Badges); err != nil {
			featureLevels_Badges = models.Rest_Feature_Levels_Badges{
				Count: 0,
			}
		}

		key = fmt.Sprintf(models.Redis_Key_Feature_RandomPictures, guild.ID)
		if err = cacheCodec.Get(key, &featureRandomPictures); err != nil {
			featureRandomPictures = models.Rest_Feature_RandomPictures{
				Count: 0,
			}
		}

		botPrefix = helpers.GetPrefixForServer(guild.ID)

		returnGuilds = append(returnGuilds, models.Rest_Guild{
			ID:        guild.ID,
			Name:      guild.Name,
			Icon:      guild.Icon,
			OwnerID:   guild.OwnerID,
			JoinedAt:  joinedAt,
			BotPrefix: botPrefix,
			Features: models.Rest_Guild_Features{
				Levels_Badges:  featureLevels_Badges,
				RandomPictures: featureRandomPictures,
			},
		})
	}

	response.WriteEntity(returnGuilds)
}

func FindUser(request *restful.Request, response *restful.Response) {
	userID := request.PathParameter("user-id")

	user, _ := helpers.GetUser(userID)
	if user != nil && user.ID != "" {
		returnUser := &models.Rest_User{
			ID:            user.ID,
			Username:      user.Username,
			AvatarHash:    user.Avatar,
			Discriminator: user.Discriminator,
			Bot:           user.Bot,
		}

		response.WriteEntity(returnUser)
	} else {
		response.WriteError(http.StatusNotFound, errors.New("User not found."))
	}
}

func FindMember(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	userID := request.PathParameter("user-id")

	member, _ := helpers.GetGuildMember(guildID, userID)
	if member != nil && member.GuildID != "" {
		joinedAt, err := discordgo.Timestamp(member.JoinedAt).Parse()
		if err != nil {
			joinedAt = time.Now()
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
		}

		returnUser := &models.Rest_Member{
			GuildID:  member.GuildID,
			JoinedAt: joinedAt,
			Nick:     member.Nick,
			Roles:    member.Roles,
		}

		response.WriteEntity(returnUser)
	} else {
		response.WriteError(http.StatusNotFound, errors.New("Member not found."))
	}
}

func IsMember(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	userID := request.PathParameter("user-id")

	if helpers.GetIsInGuild(guildID, userID) {
		response.WriteEntity(&models.Rest_Is_Member{
			IsMember: true,
		})
	} else {
		response.WriteEntity(&models.Rest_Is_Member{
			IsMember: false,
		})
	}
}

func GetProfile(request *restful.Request, response *restful.Response) {
	userID := request.PathParameter("user-id")
	guildID := request.PathParameter("guild-id")

	if guildID == "global" {
		user, err := helpers.GetUser(userID)
		if err != nil || user == nil || user.ID == "" {
			response.WriteError(http.StatusNotFound, errors.New("Profile not found."))
			return
		}

		fakeGuild := new(discordgo.Guild)
		fakeGuild.ID = "global"
		fakeMember := new(discordgo.Member)
		fakeMember.GuildID = "global"
		fakeMember.User = user

		profileHtml, err := generator.GetProfileGenerator().GetProfileHTML(fakeMember, fakeGuild, true)
		if err != nil {
			response.WriteError(http.StatusInternalServerError, err)
			return
		}
		response.Write([]byte(profileHtml))
	} else {
		guild, err := helpers.GetGuild(guildID)
		if err != nil || guild == nil || guild.ID == "" {
			response.WriteError(http.StatusNotFound, errors.New("Profile not found."))
			return
		}
		member, err := helpers.GetGuildMember(guildID, userID)
		if err != nil || member == nil || member.User == nil || member.User.ID == "" {
			response.WriteError(http.StatusNotFound, errors.New("Profile not found."))
			return
		}

		profileHtml, err := generator.GetProfileGenerator().GetProfileHTML(member, guild, true)
		if err != nil {
			response.WriteError(http.StatusInternalServerError, err)
			return
		}
		response.Write([]byte(profileHtml))
	}
}

func GetRankings(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")

	if guildID != "global" {
		guild, err := helpers.GetGuild(guildID)
		if err != nil || guild == nil || guild.ID == "" {
			response.WriteError(http.StatusNotFound, errors.New("Guild not found"))
			return
		}
	}

	var err error
	var rankingsCount int
	rankingsCountKey := fmt.Sprintf("robyul2-discord:levels:ranking:%s:by-rank:count", guildID)
	cacheCodec := cache.GetRedisCacheCodec()

	if err = cacheCodec.Get(rankingsCountKey, &rankingsCount); err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	result := new(models.Rest_Ranking)
	result.Ranks = make([]models.Rest_Ranking_Rank_Item, 0)
	result.Count = rankingsCount

	// TODO: i stuff
	i := 1
	var keyByRank string
	var rankingItem plugins.Levels_Cache_Ranking_Item
	var userItem models.Rest_User
	var isMember bool
	for {
		if i > rankingsCount {
			break
		}
		keyByRank = fmt.Sprintf("robyul2-discord:levels:ranking:%s:by-rank:%d", guildID, i)
		if err = cacheCodec.Get(keyByRank, &rankingItem); err != nil {
			break
		}
		var user *discordgo.User
		if guildID != "global" {
			member, _ := helpers.GetGuildMemberWithoutApi(guildID, rankingItem.UserID)
			if member != nil && member.User != nil && member.User.ID != "" {
				user = member.User
			} else {
				user, _ = helpers.GetUser(rankingItem.UserID)
			}
		} else {
			user, _ = helpers.GetUser(rankingItem.UserID)
		}
		if user != nil && user.ID != "" {
			userItem = models.Rest_User{
				ID:            user.ID,
				Username:      user.Username,
				AvatarHash:    user.Avatar,
				Discriminator: user.Discriminator,
				Bot:           user.Bot,
			}

			if guildID == "global" {
				isMember = true
			} else {
				isMember = true
				if !helpers.GetIsInGuild(guildID, user.ID) {
					isMember = false
				}
			}

			result.Ranks = append(result.Ranks, models.Rest_Ranking_Rank_Item{
				User:     userItem,
				EXP:      rankingItem.EXP,
				Level:    rankingItem.Level,
				Ranking:  i,
				IsMember: isMember,
			})
		}
		i += 1
		if i > 100 {
			break
		}
	}

	response.WriteEntity(result)
}

func GetUserRanking(request *restful.Request, response *restful.Response) {
	userID := request.PathParameter("user-id")
	guildID := request.PathParameter("guild-id")

	if guildID != "global" {
		guild, err := helpers.GetGuild(guildID)
		if err != nil || guild == nil || guild.ID == "" {
			response.WriteError(http.StatusNotFound, errors.New("Guild not found"))
			return
		}
	}

	var err error
	var rankingItem plugins.Levels_Cache_Ranking_Item
	rankingsKey := fmt.Sprintf("robyul2-discord:levels:ranking:%s:by-user:%s", guildID, userID)
	cacheCodec := cache.GetRedisCacheCodec()

	if err = cacheCodec.Get(rankingsKey, &rankingItem); err != nil {
		response.WriteError(http.StatusNotFound, errors.New("Member not found."))
		return
	}

	user, _ := helpers.GetUser(userID)
	if user == nil || user.ID == "" {
		response.WriteError(http.StatusNotFound, errors.New("User not found."))
		return
	}

	var isMember bool
	if guildID == "global" {
		isMember = true
	} else {
		isMember = true
		if !helpers.GetIsInGuild(guildID, user.ID) {
			isMember = false
		}
	}

	userItem := models.Rest_User{
		ID:            user.ID,
		Username:      user.Username,
		AvatarHash:    user.Avatar,
		Discriminator: user.Discriminator,
		Bot:           user.Bot,
	}
	result := models.Rest_Ranking_Rank_Item{
		User:     userItem,
		EXP:      rankingItem.EXP,
		Level:    rankingItem.Level,
		Ranking:  rankingItem.Ranking,
		IsMember: isMember,
	}

	response.WriteEntity(result)
}

func FindGuild(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")

	cacheCodec := cache.GetRedisCacheCodec()
	var key string
	var featureLevels_Badges models.Rest_Feature_Levels_Badges
	var featureRandomPictures models.Rest_Feature_RandomPictures
	var botPrefix string
	guild, _ := helpers.GetGuild(guildID)
	if guild != nil && guild.ID != "" {
		joinedAt, err := guild.JoinedAt.Parse()
		if err != nil {
			joinedAt = time.Now()
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
		}

		key = fmt.Sprintf(models.Redis_Key_Feature_Levels_Badges, guild.ID)
		if err = cacheCodec.Get(key, &featureLevels_Badges); err != nil {
			featureLevels_Badges = models.Rest_Feature_Levels_Badges{
				Count: 0,
			}
		}

		key = fmt.Sprintf(models.Redis_Key_Feature_RandomPictures, guild.ID)
		if err = cacheCodec.Get(key, &featureRandomPictures); err != nil {
			featureRandomPictures = models.Rest_Feature_RandomPictures{
				Count: 0,
			}
		}

		botPrefix = helpers.GetPrefixForServer(guild.ID)

		returnGuild := &models.Rest_Guild{
			ID:        guild.ID,
			Name:      guild.Name,
			Icon:      guild.Icon,
			OwnerID:   guild.OwnerID,
			JoinedAt:  joinedAt,
			BotPrefix: botPrefix,
			Features: models.Rest_Guild_Features{
				Levels_Badges:  featureLevels_Badges,
				RandomPictures: featureRandomPictures,
			},
		}

		response.WriteEntity(returnGuild)
	} else {
		response.WriteError(http.StatusNotFound, errors.New("Guild not found."))
	}
}

func GetRandomPicturesGuildHistory(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	startString := request.PathParameter("start")
	start, err := strconv.Atoi(startString)
	if err != nil {
		response.WriteError(http.StatusBadRequest, errors.New("Invalid arguments."))
		return
	}
	endString := request.PathParameter("end")
	end, err := strconv.Atoi(endString)
	if err != nil {
		response.WriteError(http.StatusBadRequest, errors.New("Invalid arguments."))
		return
	}

	redis := cache.GetRedisClient()
	key := fmt.Sprintf("robyul2-discord:randompictures:history:%s", guildID)

	result, err := redis.LRange(key, int64(start-1), int64(end-1)).Result()
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	resultItems := make([]models.Rest_RandomPictures_HistoryItem, 0)
	var item plugins.RandomPictures_HistoryItem
	for _, itemString := range result {
		item = plugins.RandomPictures_HistoryItem{}
		err := msgpack.Unmarshal([]byte(itemString), &item)
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			continue
		}
		resultItems = append(resultItems, models.Rest_RandomPictures_HistoryItem{
			Link:      item.Link,
			SourceID:  item.SourceID,
			PictureID: item.PictureID,
			Filename:  item.Filename,
			GuildID:   item.GuildID,
			Time:      item.Time,
		})
	}

	response.WriteEntity(resultItems)
}
