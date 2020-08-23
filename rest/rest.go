package rest

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"strings"

	"encoding/json"

	"encoding/base64"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/generator"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/modules/plugins"
	"github.com/Seklfreak/Robyul2/modules/plugins/levels"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	restful "github.com/emicklei/go-restful"
	raven "github.com/getsentry/raven-go"
	"github.com/olivere/elastic"
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
	service.Route(service.GET("").Filter(webkeyAuthenticate).To(GetAllBotGuilds))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/user").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{user-id}").Filter(sessionAndWebkeyAuthenticate).To(FindUser))
	service.Route(service.GET("/{user-id}/guilds").Filter(webkeyAuthenticate).To(FindUserGuilds))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/member").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}/{user-id}").Filter(webkeyAuthenticate).To(FindMember))
	service.Route(service.GET("/{guild-id}/{user-id}/is").Filter(webkeyAuthenticate).To(IsMember))
	service.Route(service.GET("/{guild-id}/{user-id}/status").Filter(webkeyAuthenticate).To(StatusMember))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/profile").
		Consumes(restful.MIME_JSON).
		Produces("text/html")

	service.Route(service.GET("/{user-id}/{guild-id}").Filter(webkeyAuthenticate).To(GetProfile))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/rankings").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}").Filter(webkeyAuthenticate).To(GetRankings))
	service.Route(service.GET("/user/{user-id}/{guild-id}").Filter(webkeyAuthenticate).To(GetUserRanking))
	service.Route(service.GET("/user/{user-id}/all").Filter(webkeyAuthenticate).To(GetAllUserRanking))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/guild").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}").Filter(sessionAndWebkeyAuthenticate).To(FindGuild))
	service.Route(service.POST("/{guild-id}/set-settings").Filter(sessionAndWebkeyAuthenticate).To(SetGuildSettings).Reads(&models.Rest_Receive_SetSettings{}))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/randompictures").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/history/{guild-id}/{start}/{end}").Filter(webkeyAuthenticate).To(GetRandomPicturesGuildHistory))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/statistics").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}/messages/{interval}/count").Filter(sessionAndWebkeyAuthenticate).To(GetMessageStatisticsCount))
	service.Route(service.GET("/{guild-id}/joins/{interval}/count").Filter(sessionAndWebkeyAuthenticate).To(GetJoinsStatisticsCount))
	service.Route(service.GET("/{guild-id}/leaves/{interval}/count").Filter(sessionAndWebkeyAuthenticate).To(GetLeavesStatisticsCount))
	service.Route(service.GET("/{guild-id}/by-uniques/{interval}/count").Filter(sessionAndWebkeyAuthenticate).To(GetMessageByUniqueUsersStatisticsCount))
	service.Route(service.GET("/{guild-id}/serveractivity/{interval}/histogram/{count}").Filter(sessionAndWebkeyAuthenticate).To(GetServerActivityStatisticsHistogram))
	service.Route(service.GET("/{guild-id}/vanityinvite/{interval}/histogram/{count}").Filter(sessionAndWebkeyAuthenticate).To(GetVanityInviteStatistics))
	service.Route(service.GET("/bot").Filter(webkeyAuthenticate).To(GotBotStatistics))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/chatlog").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}/{channel-id}/around/{message-id}").Filter(sessionAndWebkeyAuthenticate).To(GetChatlogAroundMessageID))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/eventlog").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{guild-id}").Filter(sessionAndWebkeyAuthenticate).To(GetEventlog))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/vanityinvite").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{vanity-name}").Filter(webkeyAuthenticate).To(GetVanityInviteByName))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/file").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	service.Route(service.GET("/{filehash}").Filter(webkeyAuthenticate).To(GetFileByFilehash))
	services = append(services, service)

	service = new(restful.WebService)
	service.
		Path("/backgrounds").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	service.Route(service.GET("").Filter(webkeyAuthenticate).To(GetAllBackgrounds))
	services = append(services, service)

	service = new(restful.WebService)
	service.Route(service.GET("/ping").Filter(webkeyAuthenticate).To(Ping))
	services = append(services, service)

	return services
}

func webkeyAuthenticate(request *restful.Request, response *restful.Response, chain *restful.FilterChain) {
	authorizationHeader := strings.TrimSpace(request.HeaderParameter("Authorization"))

	isAuthenticated := false

	if strings.HasPrefix(authorizationHeader, "Webkey ") {
		webkey := strings.TrimSpace(strings.Replace(authorizationHeader, "Webkey ", "", -1))
		if webkey == helpers.GetConfig().Path("website.webkey").Data().(string) {
			isAuthenticated = true
			request.SetAttribute("UserID", "global")
		}
	}

	queryWebkey := strings.TrimSpace(request.QueryParameter("webkey"))
	if queryWebkey == helpers.GetConfig().Path("website.webkey").Data().(string) {
		isAuthenticated = true
		request.SetAttribute("UserID", "global")
	}

	if isAuthenticated == false {
		response.WriteErrorString(401, "401: Not Authorized")
		return
	}

	chain.ProcessFilter(request, response)
	return
}

func sessionAndWebkeyAuthenticate(request *restful.Request, response *restful.Response, chain *restful.FilterChain) {
	authorizationHeader := strings.TrimSpace(request.HeaderParameter("Authorization"))

	isAuthenticated := false

	if strings.HasPrefix(authorizationHeader, "Webkey ") {
		webkey := strings.TrimSpace(strings.Replace(authorizationHeader, "Webkey ", "", -1))
		if webkey == helpers.GetConfig().Path("website.webkey").Data().(string) {
			isAuthenticated = true
			request.SetAttribute("UserID", "global")
		}
	}

	queryWebkey := strings.TrimSpace(request.QueryParameter("webkey"))
	if queryWebkey == helpers.GetConfig().Path("website.webkey").Data().(string) {
		isAuthenticated = true
		request.SetAttribute("UserID", "global")
	}

	if strings.HasPrefix(authorizationHeader, "PHP-Session ") {
		sessionID := strings.TrimSpace(strings.Replace(authorizationHeader, "PHP-Session ", "", -1))
		key := "robyul2-web:robyul-session:" + sessionID
		redis := cache.GetRedisClient()
		sessionDataString, err := redis.Get(key).Result()
		if err == nil {
			var sessionData models.Website_Session_Data
			msgpack.Unmarshal([]byte(sessionDataString), &sessionData)
			if sessionData.DiscordUserID != "" {
				isAuthenticated = true
				request.SetAttribute("UserID", sessionData.DiscordUserID)
			}
		}
	}

	if isAuthenticated == false {
		response.WriteErrorString(401, "401: Not Authorized")
		return
	}

	chain.ProcessFilter(request, response)
	return
}

func GetAllBotGuilds(request *restful.Request, response *restful.Response) {
	var botPrefix string

	returnGuilds := make([]models.Rest_Guild, 0)
	for _, shard := range cache.GetSession().Sessions {
		for _, guild := range shard.State.Guilds {
			joinedAt := helpers.GetTimeFromSnowflake(guild.ID)
			botPrefix = helpers.GetPrefixForServer(guild.ID)

			returnGuilds = append(returnGuilds, models.Rest_Guild{
				ID:        guild.ID,
				Name:      guild.Name,
				Icon:      guild.Icon,
				OwnerID:   guild.OwnerID,
				JoinedAt:  joinedAt,
				BotPrefix: botPrefix,
				Features:  getGuildFeatures(guild.ID),
				Settings:  getGuildSettings(guild.ID, request.Attribute("UserID").(string)),
			})
		}
	}

	response.WriteEntity(returnGuilds)
}

func FindUser(request *restful.Request, response *restful.Response) {
	userID := request.PathParameter("user-id")

	if request.Attribute("UserID").(string) != "global" && request.Attribute("UserID").(string) != userID {
		response.WriteErrorString(401, "401: Not Authorized")
		return
	}

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

func FindUserGuilds(request *restful.Request, response *restful.Response) {
	userID := request.PathParameter("user-id")

	var botPrefix string

	returnGuilds := make([]models.Rest_Member_Guild, 0)
	for _, shard := range cache.GetSession().Sessions {
		for _, guild := range shard.State.Guilds {
			if !helpers.GetIsInGuild(guild.ID, userID) {
				continue
			}

			joinedAt := helpers.GetTimeFromSnowflake(guild.ID)

			botPrefix = helpers.GetPrefixForServer(guild.ID)

			returnStatus := models.Rest_Status_Member{}
			returnStatus.IsMember = true
			if helpers.IsBotAdmin(userID) {
				returnStatus.IsBotAdmin = true
			}
			if helpers.IsNukeMod(userID) {
				returnStatus.IsNukeMod = true
			}
			if helpers.IsRobyulMod(userID) {
				returnStatus.IsRobyulStaff = true
			}
			if helpers.IsBlacklisted(userID) {
				returnStatus.IsBlacklisted = true
			}
			if helpers.IsAdminByID(guild.ID, userID) {
				returnStatus.IsGuildAdmin = true
			}
			if helpers.IsModByID(guild.ID, userID) {
				returnStatus.IsGuildMod = true
			}
			if helpers.HasPermissionByID(guild.ID, userID, discordgo.PermissionAdministrator) {
				returnStatus.HasGuildPermissionAdministrator = true
			}

			returnGuilds = append(returnGuilds, models.Rest_Member_Guild{
				ID:        guild.ID,
				Name:      guild.Name,
				Icon:      guild.Icon,
				OwnerID:   guild.OwnerID,
				JoinedAt:  joinedAt,
				BotPrefix: botPrefix,
				Features:  getGuildFeatures(guild.ID),
				Settings:  getGuildSettings(guild.ID, request.Attribute("UserID").(string)),
				Status:    returnStatus,
			})
		}
	}

	response.WriteEntity(returnGuilds)
}

func FindMember(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	userID := request.PathParameter("user-id")

	member, _ := helpers.GetGuildMemberWithoutApi(guildID, userID)
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

func StatusMember(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	userID := request.PathParameter("user-id")

	returnStatus := &models.Rest_Status_Member{}

	if helpers.GetIsInGuild(guildID, userID) {
		returnStatus.IsMember = true
		if helpers.IsBotAdmin(userID) {
			returnStatus.IsBotAdmin = true
		}
		if helpers.IsNukeMod(userID) {
			returnStatus.IsNukeMod = true
		}
		if helpers.IsRobyulMod(userID) {
			returnStatus.IsRobyulStaff = true
		}
		if helpers.IsBlacklisted(userID) {
			returnStatus.IsBlacklisted = true
		}
		if helpers.IsAdminByID(guildID, userID) {
			returnStatus.IsGuildAdmin = true
		}
		if helpers.IsModByID(guildID, userID) {
			returnStatus.IsGuildMod = true
		}
		if helpers.HasPermissionByID(guildID, userID, discordgo.PermissionAdministrator) {
			returnStatus.HasGuildPermissionAdministrator = true
		}
	}
	response.WriteEntity(returnStatus)
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
		member, err := helpers.GetGuildMemberWithoutApi(guildID, userID)
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
	var rankingItem levels.Levels_Cache_Ranking_Item
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
				user, _ = helpers.GetUserWithoutAPI(rankingItem.UserID)
			}
		} else {
			user, _ = helpers.GetUserWithoutAPI(rankingItem.UserID)
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

			expForLevel := levels.GetExpForLevel(levels.GetLevelFromExp(rankingItem.EXP))

			result.Ranks = append(result.Ranks, models.Rest_Ranking_Rank_Item{
				User:                userItem,
				GuildID:             guildID,
				IsMember:            isMember,
				EXP:                 rankingItem.EXP,
				Level:               rankingItem.Level,
				Ranking:             i,
				NextLevelCurrentEXP: rankingItem.EXP - expForLevel,
				NextLevelTotalEXP:   levels.GetExpForLevel(levels.GetLevelFromExp(rankingItem.EXP)+1) - expForLevel,
				Progress:            levels.GetProgressToNextLevelFromExp(rankingItem.EXP),
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
	var rankingItem levels.Levels_Cache_Ranking_Item
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

	expForLevel := levels.GetExpForLevel(levels.GetLevelFromExp(rankingItem.EXP))

	result := models.Rest_Ranking_Rank_Item{
		User:                userItem,
		EXP:                 rankingItem.EXP,
		Level:               rankingItem.Level,
		Ranking:             rankingItem.Ranking,
		IsMember:            isMember,
		GuildID:             guildID,
		NextLevelCurrentEXP: rankingItem.EXP - expForLevel,
		NextLevelTotalEXP:   levels.GetExpForLevel(levels.GetLevelFromExp(rankingItem.EXP)+1) - expForLevel,
		Progress:            levels.GetProgressToNextLevelFromExp(rankingItem.EXP),
	}

	response.WriteEntity(result)
}

func GetAllUserRanking(request *restful.Request, response *restful.Response) {
	userID := request.PathParameter("user-id")

	var err error

	var rankingItem levels.Levels_Cache_Ranking_Item
	cacheCodec := cache.GetRedisCacheCodec()

	user, _ := helpers.GetUser(userID)
	if user == nil || user.ID == "" {
		response.WriteError(http.StatusNotFound, errors.New("User not found."))
		return
	}
	userItem := models.Rest_User{
		ID:            user.ID,
		Username:      user.Username,
		AvatarHash:    user.Avatar,
		Discriminator: user.Discriminator,
		Bot:           user.Bot,
	}

	result := make([]models.Rest_Ranking_Rank_Item, 0)

	for _, guild := range append(helpers.AllGuilds(), &discordgo.Guild{ID: "global", Name: "global"}) {
		if guild.ID != "global" && !helpers.GetIsInGuild(guild.ID, userID) {
			continue
		}

		rankingsKey := fmt.Sprintf("robyul2-discord:levels:ranking:%s:by-user:%s", guild.ID, userID)
		if err = cacheCodec.Get(rankingsKey, &rankingItem); err != nil {
			continue
		}

		expForLevel := levels.GetExpForLevel(levels.GetLevelFromExp(rankingItem.EXP))

		result = append(result, models.Rest_Ranking_Rank_Item{
			User:                userItem,
			GuildID:             guild.ID,
			IsMember:            true,
			EXP:                 rankingItem.EXP,
			Level:               rankingItem.Level,
			Ranking:             rankingItem.Ranking,
			NextLevelCurrentEXP: rankingItem.EXP - expForLevel,
			NextLevelTotalEXP:   levels.GetExpForLevel(levels.GetLevelFromExp(rankingItem.EXP)+1) - expForLevel,
			Progress:            levels.GetProgressToNextLevelFromExp(rankingItem.EXP),
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].EXP > result[j].EXP })

	response.WriteEntity(result)
}

func FindGuild(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")

	if request.Attribute("UserID").(string) != "global" && !helpers.GetIsInGuild(guildID, request.Attribute("UserID").(string)) {
		response.WriteErrorString(401, "401: Not Authorized")
		return
	}

	var botPrefix string
	guild, _ := helpers.GetGuild(guildID)
	if guild != nil && guild.ID != "" {
		joinedAt, _ := guild.JoinedAt.Parse()

		botPrefix = helpers.GetPrefixForServer(guild.ID)

		channels := make([]models.Rest_Channel, 0)
		sort.Slice(guild.Channels, func(i, j int) bool { return guild.Channels[i].Position < guild.Channels[j].Position })
		for _, channel := range guild.Channels {
			channelType := "text"
			switch channel.Type {
			case discordgo.ChannelTypeGuildVoice:
				channelType = "voice"
			case discordgo.ChannelTypeGuildCategory:
				channelType = "category"
			}
			channels = append(channels, models.Rest_Channel{
				ID:       channel.ID,
				ParentID: channel.ParentID,
				GuildID:  channel.GuildID,
				Name:     channel.Name,
				Type:     channelType,
				Topic:    channel.Topic,
				Position: channel.Position,
			})
		}

		returnGuild := &models.Rest_Guild{
			ID:        guild.ID,
			Name:      guild.Name,
			Icon:      guild.Icon,
			OwnerID:   guild.OwnerID,
			JoinedAt:  joinedAt,
			BotPrefix: botPrefix,
			Features:  getGuildFeatures(guild.ID),
			Channels:  channels,
			Settings:  getGuildSettings(guild.ID, request.Attribute("UserID").(string)),
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

func GetMessageStatisticsCount(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	interval := request.PathParameter("interval")

	if request.Attribute("UserID").(string) != "global" {
		if !helpers.IsModByID(guildID, request.Attribute("UserID").(string)) && !helpers.IsAdminByID(guildID, request.Attribute("UserID").(string)) {
			response.WriteErrorString(401, "401: Not Authorized")
			return
		}
	}

	if !cache.HasElastic() {
		response.WriteErrorString(http.StatusServiceUnavailable, "unavailable")
		return
	}

	rangeQuery := elastic.NewRangeQuery("CreatedAt").
		Gte("now-" + interval).
		Lte("now")
	termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID)
	finalQuery := elastic.NewBoolQuery().Must(rangeQuery, termQuery)
	searchResult, err := cache.GetElastic().Count().
		Index(models.ElasticIndexMessages).
		Type("doc").
		Query(finalQuery).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	response.WriteEntity(models.Rest_Statistics_Count{Count: searchResult})
}

func GetJoinsStatisticsCount(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	interval := request.PathParameter("interval")

	if request.Attribute("UserID").(string) != "global" {
		if !helpers.IsModByID(guildID, request.Attribute("UserID").(string)) && !helpers.IsAdminByID(guildID, request.Attribute("UserID").(string)) {
			response.WriteErrorString(401, "401: Not Authorized")
			return
		}
	}

	if !cache.HasElastic() {
		response.WriteErrorString(http.StatusServiceUnavailable, "unavailable")
		return
	}

	rangeQuery := elastic.NewRangeQuery("CreatedAt").
		Gte("now-" + interval).
		Lte("now")
	termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID)
	finalQuery := elastic.NewBoolQuery().Must(rangeQuery, termQuery)
	searchResult, err := cache.GetElastic().Count().
		Index(models.ElasticIndexJoins).
		Type("doc").
		Query(finalQuery).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	response.WriteEntity(models.Rest_Statistics_Count{Count: searchResult})
}

func GetLeavesStatisticsCount(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	interval := request.PathParameter("interval")

	if request.Attribute("UserID").(string) != "global" {
		if !helpers.IsModByID(guildID, request.Attribute("UserID").(string)) && !helpers.IsAdminByID(guildID, request.Attribute("UserID").(string)) {
			response.WriteErrorString(401, "401: Not Authorized")
			return
		}
	}

	if !cache.HasElastic() {
		response.WriteErrorString(http.StatusServiceUnavailable, "unavailable")
		return
	}

	rangeQuery := elastic.NewRangeQuery("CreatedAt").
		Gte("now-" + interval).
		Lte("now")
	termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID)
	finalQuery := elastic.NewBoolQuery().Must(rangeQuery, termQuery)
	searchResult, err := cache.GetElastic().Count().
		Index(models.ElasticIndexLeaves).
		Type("doc").
		Query(finalQuery).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	response.WriteEntity(models.Rest_Statistics_Count{Count: searchResult})
}

func GetMessageByUniqueUsersStatisticsCount(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	interval := request.PathParameter("interval")

	if request.Attribute("UserID").(string) != "global" {
		if !helpers.IsModByID(guildID, request.Attribute("UserID").(string)) && !helpers.IsAdminByID(guildID, request.Attribute("UserID").(string)) {
			response.WriteErrorString(401, "401: Not Authorized")
			return
		}
	}

	if !cache.HasElastic() {
		response.WriteErrorString(http.StatusServiceUnavailable, "unavailable")
		return
	}

	agg := elastic.NewCardinalityAggregation().
		Field("UserID.keyword")

	rangeQuery := elastic.NewRangeQuery("CreatedAt").
		Gte("now-" + interval).
		Lte("now")
	termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID)
	finalQuery := elastic.NewBoolQuery().Must(rangeQuery, termQuery)
	searchResult, err := cache.GetElastic().Search().
		Index(models.ElasticIndexMessages).
		Type("doc").
		Query(finalQuery).
		Aggregation("distinct_user_ids", agg).
		Size(0).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	var result int64
	if agg, found := searchResult.Aggregations.Terms("distinct_user_ids"); found {
		if raw, found := agg.Aggregations["value"]; found {
			if raw == nil {
				response.WriteError(http.StatusInternalServerError, errors.New("invalid storage response"))
				return
			}

			result, err = strconv.ParseInt(string(*raw), 10, 64)
			if err != nil {
				response.WriteError(http.StatusInternalServerError, err)
				return
			}
		}
	}

	response.WriteEntity(models.Rest_Statistics_Count{Count: result})
}

func GetServerActivityStatisticsHistogram(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	interval := request.PathParameter("interval")
	count := request.PathParameter("count")

	if request.Attribute("UserID").(string) != "global" {
		if !helpers.IsModByID(guildID, request.Attribute("UserID").(string)) && !helpers.IsAdminByID(guildID, request.Attribute("UserID").(string)) {
			response.WriteErrorString(401, "401: Not Authorized")
			return
		}
	}

	if !cache.HasElastic() {
		response.WriteErrorString(http.StatusServiceUnavailable, "unavailable")
		return
	}

	countNumber, err := strconv.Atoi(count)
	if err != nil {
		response.WriteError(http.StatusNoContent, errors.New("invalid count"))
		return
	}

	minBound := helpers.GetMinTimeForInterval(interval, countNumber)

	agg := elastic.NewDateHistogramAggregation().
		Field("CreatedAt").
		Interval(interval).
		Order("_key", false).
		MinDocCount(0).
		ExtendedBoundsMin(minBound).
		ExtendedBoundsMax(time.Now())

	aggWithUniqueUsers := agg.
		SubAggregation(
			"distinct_user_ids",
			elastic.NewCardinalityAggregation().
				Field("UserID.keyword"),
		)

	termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID)
	searchResult, err := cache.GetElastic().Search().
		Index(models.ElasticIndexMessages).
		Type("doc").
		Query(termQuery).
		Aggregation("messages", aggWithUniqueUsers).
		Size(0).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	result := make([]models.Rest_Statistics_Histogram_Three, 0)

	var timestamp int64
	var timeConverted time.Time
	var timeISO8601 string
	var distinctUserIDs float64
	if agg, found := searchResult.Aggregations.Terms("messages"); found {
		for _, bucket := range agg.Buckets {
			timestamp = int64(bucket.Key.(float64) / 1000)
			timeConverted = time.Unix(timestamp, 0)
			timeISO8601 = timeConverted.Format(models.ISO8601)

			distinctUserIDs = 0
			distinctUserIDsValue, exists := bucket.ValueCount("distinct_user_ids")
			if exists {
				distinctUserIDs = *distinctUserIDsValue.Value
			}

			result = append(result, models.Rest_Statistics_Histogram_Three{
				Time:   timeISO8601,
				Count1: bucket.DocCount,
				Count2: 0,
				Count3: 0,
				Count4: int64(distinctUserIDs),
			})

			if len(result) >= countNumber {
				break
			}
		}
	}

	termQuery = elastic.NewQueryStringQuery("GuildID:" + guildID)
	searchResult, err = cache.GetElastic().Search().
		Index(models.ElasticIndexJoins).
		Type("doc").
		Query(termQuery).
		Aggregation("joins", agg).
		Size(0).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	if agg, found := searchResult.Aggregations.Terms("joins"); found {
		for _, bucket := range agg.Buckets {
			timestamp = int64(bucket.Key.(float64) / 1000)
			timeConverted = time.Unix(timestamp, 0)
			timeISO8601 = timeConverted.Format(models.ISO8601)

			for resultN := range result {
				if result[resultN].Time == timeISO8601 {
					result[resultN].Count2 = bucket.DocCount
				}
			}
		}
	}

	termQuery = elastic.NewQueryStringQuery("GuildID:" + guildID)
	searchResult, err = cache.GetElastic().Search().
		Index(models.ElasticIndexLeaves).
		Type("doc").
		Query(termQuery).
		Aggregation("leaves", agg).
		Size(0).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	if agg, found := searchResult.Aggregations.Terms("leaves"); found {
		for _, bucket := range agg.Buckets {
			timestamp = int64(bucket.Key.(float64) / 1000)
			timeConverted = time.Unix(timestamp, 0)
			timeISO8601 = timeConverted.Format(models.ISO8601)

			for resultN := range result {
				if result[resultN].Time == timeISO8601 {
					result[resultN].Count3 = bucket.DocCount
				}
			}
		}
	}

	response.WriteEntity(result)
}

func GetVanityInviteStatistics(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	interval := request.PathParameter("interval")
	count := request.PathParameter("count")

	if request.Attribute("UserID").(string) != "global" {
		if !helpers.IsModByID(guildID, request.Attribute("UserID").(string)) && !helpers.IsAdminByID(guildID, request.Attribute("UserID").(string)) {
			response.WriteErrorString(401, "401: Not Authorized")
			return
		}
	}

	if !cache.HasElastic() {
		response.WriteErrorString(http.StatusServiceUnavailable, "unavailable")
		return
	}

	countNumber, err := strconv.Atoi(count)
	if err != nil {
		response.WriteError(http.StatusNoContent, errors.New("invalid count"))
		return
	}

	vanityInvite, _ := helpers.GetVanityUrlByGuildID(guildID)
	if vanityInvite.VanityName == "" {
		response.WriteError(http.StatusNoContent, errors.New("vanity invite not found"))
		return
	}

	minBound := helpers.GetMinTimeForInterval(interval, countNumber)

	refererAgg := elastic.NewTermsAggregation().
		Field("Referer.keyword").
		Order("_count", false)

	agg := elastic.NewDateHistogramAggregation().
		Field("CreatedAt").
		Interval(interval).
		Order("_key", false).
		MinDocCount(0).
		ExtendedBoundsMin(minBound).
		ExtendedBoundsMax(time.Now())

	combinedAgg := agg.SubAggregation("referers", refererAgg)

	termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID)
	searchResult, err := cache.GetElastic().Search().
		Index(models.ElasticIndexVanityInviteClicks).
		Type("doc").
		Query(termQuery).
		Aggregation("clicks", combinedAgg).
		Size(0).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	result := make([]models.Rest_Statistics_Histogram_TwoSub, 0)

	var timestamp int64
	var timeConverted time.Time
	var timeISO8601 string
	if agg, found := searchResult.Aggregations.Terms("clicks"); found {
		for _, bucket := range agg.Buckets {
			timestamp = int64(bucket.Key.(float64) / 1000)
			timeConverted = time.Unix(timestamp, 0)
			timeISO8601 = timeConverted.Format(models.ISO8601)

			subItems := make([]models.Rest_Statistics_Histogram_TwoSub_SubItem, 0)

			if subAgg, subFound := bucket.Aggregations.Terms("referers"); subFound {
				for _, subBucket := range subAgg.Buckets {
					referer := subBucket.Key.(string)
					//fmt.Println("refers sub bucket", referer, subBucket.DocCount)
					subItems = append(subItems, models.Rest_Statistics_Histogram_TwoSub_SubItem{
						Key:   referer,
						Value: subBucket.DocCount,
					})
				}
			}

			//fmt.Println("clicks bucket", timeISO8601+":", bucket.DocCount)
			result = append(result, models.Rest_Statistics_Histogram_TwoSub{
				Time:     timeISO8601,
				Count1:   bucket.DocCount,
				Count2:   0,
				SubItems: subItems,
			})
			if len(result) >= countNumber {
				break
			}
		}
	}

	termQuery = elastic.NewQueryStringQuery("GuildID:" + guildID + " AND VanityInvite:" + vanityInvite.VanityName)
	searchResult, err = cache.GetElastic().Search().
		Index(models.ElasticIndexJoins).
		Type("doc").
		Query(termQuery).
		Aggregation("joins", agg).
		Size(0).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	if agg, found := searchResult.Aggregations.Terms("joins"); found {
		for _, bucket := range agg.Buckets {
			timestamp = int64(bucket.Key.(float64) / 1000)
			timeConverted = time.Unix(timestamp, 0)
			timeISO8601 = timeConverted.Format(models.ISO8601)
			//fmt.Println("joins bucket", timeISO8601+":", bucket.DocCount)
			//result[n].Count2 = bucket.DocCount
			for resultN := range result {
				if result[resultN].Time == timeISO8601 {
					result[resultN].Count2 = bucket.DocCount
				}
			}
		}
	}

	response.WriteEntity(result)
}

func GotBotStatistics(request *restful.Request, response *restful.Response) {
	users := make(map[string]string)
	var guildCount int

	for _, shard := range cache.GetSession().Sessions {
		for _, guild := range shard.State.Guilds {
			guildCount++
			for _, u := range guild.Members {
				users[u.User.ID] = u.User.Username
			}
		}
	}

	response.WriteEntity(models.Rest_Statitics_Bot{
		Guilds: guildCount,
		Users:  len(users),
	})
}

func GetChatlogAroundMessageID(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")
	channelID := request.PathParameter("channel-id")
	messageID := request.PathParameter("message-id")

	if request.Attribute("UserID").(string) != "global" {
		if !helpers.IsModByID(guildID, request.Attribute("UserID").(string)) && !helpers.HasPermissionByID(
			guildID, request.Attribute("UserID").(string), discordgo.PermissionAdministrator) {
			response.WriteErrorString(401, "401: Not Authorized")
			return
		}
	}

	if helpers.GuildSettingsGetCached(guildID).ChatlogDisabled {
		response.WriteErrorString(401, "401: Not Authorized")
		return
	}

	if !cache.HasElastic() {
		response.WriteErrorString(http.StatusServiceUnavailable, "unavailable")
		return
	}

	if messageID == "last" {
		termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID + " AND ChannelID:" + channelID)
		searchResult, err := cache.GetElastic().Search().
			Index(models.ElasticIndexMessages).
			Type("doc").
			Query(termQuery).
			Size(1).
			Sort("CreatedAt", false).
			Do(context.Background())
		if err != nil {
			response.WriteError(http.StatusInternalServerError, err)
			return
		}

		for _, item := range searchResult.Hits.Hits {
			if item == nil {
				continue
			}

			m := helpers.UnmarshalElasticMessage(item)

			if m.MessageID == "" {
				continue
			}

			messageID = m.MessageID
		}
	}

	if messageID == "" {
		response.WriteError(http.StatusNoContent, errors.New("Message not found"))
		return
	}

	termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID + " AND ChannelID:" + channelID + " AND MessageID:" + messageID)
	searchResult, err := cache.GetElastic().Search().
		Index(models.ElasticIndexMessages).
		Type("doc").
		Query(termQuery).
		Size(1).
		Sort("CreatedAt", true).
		//Sort("MessageID.keyword", true).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	if searchResult.TotalHits() <= 0 {
		response.WriteError(http.StatusNoContent, errors.New("Message not found"))
		return
	}

	result := make([]models.Rest_Chatlog_Message, 0)
	var sortValues []interface{}
	for _, item := range searchResult.Hits.Hits {
		if item == nil {
			continue
		}

		m := helpers.UnmarshalElasticMessage(item)

		if m.MessageID == "" {
			continue
		}

		author, _ := helpers.GetUserWithoutAPI(m.UserID)
		if author == nil || author.ID == "" {
			author = new(discordgo.User)
			author.Username = "N/A"
		}

		result = append(result, models.Rest_Chatlog_Message{
			CreatedAt:      m.CreatedAt,
			ID:             m.MessageID,
			Content:        m.Content,
			Attachments:    m.Attachments,
			AuthorID:       m.UserID,
			AuthorUsername: author.Username,
			Embeds:         m.Embeds,
			Deleted:        m.Deleted,
		})

		sortValues = item.Sort
	}

	termQuery = elastic.NewQueryStringQuery("GuildID:" + guildID + " AND ChannelID:" + channelID)
	searchResult, err = cache.GetElastic().Search().
		Index(models.ElasticIndexMessages).
		Type("doc").
		Query(termQuery).
		Size(50).
		SearchAfter(sortValues[0]).
		Sort("CreatedAt", true).
		//Sort("MessageID.keyword", true).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	for _, item := range searchResult.Hits.Hits {
		if item == nil {
			continue
		}

		m := helpers.UnmarshalElasticMessage(item)

		if m.MessageID == "" {
			continue
		}

		author, _ := helpers.GetUserWithoutAPI(m.UserID)
		if author == nil || author.ID == "" {
			author = new(discordgo.User)
			author.Username = "N/A"
		}

		result = append(result, models.Rest_Chatlog_Message{
			CreatedAt:      m.CreatedAt,
			ID:             m.MessageID,
			Content:        m.Content,
			Attachments:    m.Attachments,
			AuthorID:       m.UserID,
			AuthorUsername: author.Username,
			Embeds:         m.Embeds,
			Deleted:        m.Deleted,
		})
	}

	termQuery = elastic.NewQueryStringQuery("GuildID:" + guildID + " AND ChannelID:" + channelID)
	searchResult, err = cache.GetElastic().Search().
		Index(models.ElasticIndexMessages).
		Type("doc").
		Query(termQuery).
		Size(50).
		SearchAfter(sortValues[0]).
		Sort("CreatedAt", false).
		//Sort("MessageID.keyword", false).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	for _, item := range searchResult.Hits.Hits {
		if item == nil {
			continue
		}

		m := helpers.UnmarshalElasticMessage(item)

		if m.MessageID == "" {
			continue
		}

		author, _ := helpers.GetUserWithoutAPI(m.UserID)
		if author == nil || author.ID == "" {
			author = new(discordgo.User)
			author.Username = "N/A"
		}

		result = append([]models.Rest_Chatlog_Message{{
			CreatedAt:      m.CreatedAt,
			ID:             m.MessageID,
			Content:        m.Content,
			Attachments:    m.Attachments,
			AuthorID:       m.UserID,
			AuthorUsername: author.Username,
			Embeds:         m.Embeds,
			Deleted:        m.Deleted,
		}}, result...)
	}

	// TODO: replace with MessageID.keyword sort, need ElasticSearch reindex
	// sort by timestamp, if timestamp equal sort by message ID
	slice.Sort(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	response.WriteEntity(result)
}

func GetEventlog(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")

	if request.Attribute("UserID").(string) != "global" {
		if !helpers.IsModByID(guildID, request.Attribute("UserID").(string)) {
			response.WriteErrorString(401, "401: Not Authorized")
			return
		}
	}

	if helpers.GuildSettingsGetCached(guildID).EventlogDisabled {
		response.WriteErrorString(401, "401: Not Authorized")
		return
	}

	if !cache.HasElastic() {
		response.WriteErrorString(http.StatusServiceUnavailable, "unavailable")
		return
	}

	termQuery := elastic.NewQueryStringQuery("GuildID:" + guildID)
	searchResult, err := cache.GetElastic().Search().
		Index(models.ElasticIndexEventlogs).
		Type("doc").
		Query(termQuery).
		Size(50).
		Sort("CreatedAt", false).
		Do(context.Background())
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	if searchResult.TotalHits() <= 0 {
		response.WriteError(http.StatusNoContent, errors.New("eventlog empty"))
		return
	}

	eventlog := models.Rest_Eventlog{
		Users:   make([]models.Rest_User, 0),
		Entries: make([]models.Rest_Eventlog_Entry, 0),
	}

	lookupUserIDs := make([]string, 0)
	lookupChannelIDs := make([]string, 0)
	lookupRoleIDs := make([]string, 0)
	lookupEmojiIDs := make([]string, 0)
	lookupGuildIDs := make([]string, 0)

	var alreadyLookingUp bool
	for _, item := range searchResult.Hits.Hits {
		if item == nil {
			continue
		}

		var elasticEventlog models.ElasticEventlog
		err := json.Unmarshal(*item.Source, &elasticEventlog)
		if err != nil {
			response.WriteError(http.StatusInternalServerError, err)
			return
		}

		waitingForData := false

		if elasticEventlog.WaitingFor.AuditLogBackfill {
			waitingForData = true
		}

		eventlog.Entries = append(eventlog.Entries, models.Rest_Eventlog_Entry{
			CreatedAt:      elasticEventlog.CreatedAt.UTC(),
			TargetID:       elasticEventlog.TargetID,
			TargetType:     elasticEventlog.TargetType,
			UserID:         elasticEventlog.UserID,
			ActionType:     elasticEventlog.ActionType,
			Reason:         elasticEventlog.Reason,
			Changes:        elasticEventlog.Changes,
			Options:        elasticEventlog.Options,
			WaitingForData: waitingForData,
		})

		alreadyLookingUp = false
		switch elasticEventlog.TargetType {
		case models.EventlogTargetTypeUser:
			for _, lookupUserID := range lookupUserIDs {
				if lookupUserID == elasticEventlog.TargetID {
					alreadyLookingUp = true
				}
			}
			if !alreadyLookingUp {
				lookupUserIDs = append(lookupUserIDs, elasticEventlog.TargetID)
			}
			break
		case models.EventlogTargetTypeChannel:
			for _, lookupChannelID := range lookupChannelIDs {
				if lookupChannelID == elasticEventlog.TargetID {
					alreadyLookingUp = true
				}
			}
			if !alreadyLookingUp {
				lookupChannelIDs = append(lookupChannelIDs, elasticEventlog.TargetID)
			}
			break
		case models.EventlogTargetTypeRole:
			for _, lookupRoleID := range lookupRoleIDs {
				if lookupRoleID == elasticEventlog.TargetID {
					alreadyLookingUp = true
				}
			}
			if !alreadyLookingUp {
				lookupRoleIDs = append(lookupRoleIDs, elasticEventlog.TargetID)
			}
			break
		case models.EventlogTargetTypeEmoji:
			for _, lookupEmojiID := range lookupEmojiIDs {
				if lookupEmojiID == elasticEventlog.TargetID {
					alreadyLookingUp = true
				}
			}
			if !alreadyLookingUp {
				lookupEmojiIDs = append(lookupEmojiIDs, elasticEventlog.TargetID)
			}
			break
		case models.EventlogTargetTypeGuild:
			for _, lookupGuildID := range lookupGuildIDs {
				if lookupGuildID == elasticEventlog.TargetID {
					alreadyLookingUp = true
				}
			}
			if !alreadyLookingUp {
				lookupGuildIDs = append(lookupGuildIDs, elasticEventlog.TargetID)
			}
			break
		}

		for _, option := range elasticEventlog.Options {
			if option.Key == "member_roles_added" {
				alreadyLookingUp = false
				for _, roleID := range strings.Split(option.Value, ",") {
					for _, lookupRoleID := range lookupRoleIDs {
						if lookupRoleID == roleID {
							alreadyLookingUp = true
						}
					}
					if !alreadyLookingUp {
						lookupRoleIDs = append(lookupRoleIDs, roleID)
					}
				}
			}
			if option.Key == "member_roles_removed" {
				alreadyLookingUp = false
				for _, roleID := range strings.Split(option.Value, ",") {
					for _, lookupRoleID := range lookupRoleIDs {
						if lookupRoleID == roleID {
							alreadyLookingUp = true
						}
					}
					if !alreadyLookingUp {
						lookupRoleIDs = append(lookupRoleIDs, roleID)
					}
				}
			}
		}

		for _, change := range elasticEventlog.Changes {
			if change.Key == "member_roles" {
				alreadyLookingUp = false
				for _, roleID := range strings.Split(change.OldValue, ",") {
					for _, lookupRoleID := range lookupRoleIDs {
						if lookupRoleID == roleID {
							alreadyLookingUp = true
						}
					}
					if !alreadyLookingUp {
						lookupRoleIDs = append(lookupRoleIDs, roleID)
					}
				}
				alreadyLookingUp = false
				for _, roleID := range strings.Split(change.NewValue, ",") {
					for _, lookupRoleID := range lookupRoleIDs {
						if lookupRoleID == roleID {
							alreadyLookingUp = true
						}
					}
					if !alreadyLookingUp {
						lookupRoleIDs = append(lookupRoleIDs, roleID)
					}
				}
			}
		}

		alreadyLookingUp = false
		if elasticEventlog.UserID != "" {
			for _, lookupUserID := range lookupUserIDs {
				if lookupUserID == elasticEventlog.UserID {
					alreadyLookingUp = true
				}
			}
			if !alreadyLookingUp {
				lookupUserIDs = append(lookupUserIDs, elasticEventlog.UserID)
			}
		}
	}

	for _, lookupUserID := range lookupUserIDs {
		user, _ := helpers.GetUserWithoutAPI(lookupUserID)
		if user != nil && user.ID != "" {
			eventlog.Users = append(eventlog.Users, models.Rest_User{
				ID:            user.ID,
				Username:      user.Username,
				AvatarHash:    user.Avatar,
				Discriminator: user.Discriminator,
				Bot:           user.Bot,
			})
		}
	}

	for _, lookupChannelID := range lookupChannelIDs {
		channel, _ := helpers.GetChannelWithoutApi(lookupChannelID)
		if channel != nil && channel.ID != "" {
			channelType := "text"
			switch channel.Type {
			case discordgo.ChannelTypeGuildVoice:
				channelType = "voice"
			case discordgo.ChannelTypeGuildCategory:
				channelType = "category"
			}
			eventlog.Channels = append(eventlog.Channels, models.Rest_Channel{
				ID:       channel.ID,
				ParentID: channel.ParentID,
				GuildID:  channel.GuildID,
				Name:     channel.Name,
				Type:     channelType,
				Topic:    channel.Topic,
				Position: channel.Position,
			})
			if channel.ParentID != "" {
				alreadyLookingUp = false
				for _, lookupChannelID := range lookupChannelIDs {
					if lookupChannelID == channel.ParentID {
						alreadyLookingUp = true
					}
				}
				for _, lookedUpChannel := range eventlog.Channels {
					if lookedUpChannel.ID == channel.ParentID {
						alreadyLookingUp = true
					}
				}
				if !alreadyLookingUp {
					channel, _ := helpers.GetChannelWithoutApi(lookupChannelID)
					if channel != nil && channel.ID != "" {
						channelType := "text"
						switch channel.Type {
						case discordgo.ChannelTypeGuildVoice:
							channelType = "voice"
						case discordgo.ChannelTypeGuildCategory:
							channelType = "category"
						}
						eventlog.Channels = append(eventlog.Channels, models.Rest_Channel{
							ID:       channel.ID,
							ParentID: channel.ParentID,
							GuildID:  channel.GuildID,
							Name:     channel.Name,
							Type:     channelType,
							Topic:    channel.Topic,
							Position: channel.Position,
						})
					}
				}
			}
		}
	}

	for _, lookupRoleID := range lookupRoleIDs {
		role, _ := cache.GetSession().SessionForGuildS(guildID).State.Role(guildID, lookupRoleID)
		if role != nil && role.ID != "" {
			eventlog.Roles = append(eventlog.Roles, models.Rest_Role{
				ID:          role.ID,
				GuildID:     guildID,
				Name:        role.Name,
				Managed:     role.Managed,
				Mentionable: role.Mentionable,
				Hoist:       role.Hoist,
				Color:       helpers.GetHexFromDiscordColor(role.Color),
				Position:    role.Position,
				Permissions: role.Permissions,
			})
		}
	}

	for _, lookupEmojiID := range lookupEmojiIDs {
		emoji, _ := cache.GetSession().SessionForGuildS(guildID).State.Emoji(guildID, lookupEmojiID)
		if emoji != nil && emoji.ID != "" {
			eventlog.Emoji = append(eventlog.Emoji, models.Rest_Emoji{
				ID:            emoji.ID,
				GuildID:       guildID,
				Name:          emoji.Name,
				Managed:       emoji.Managed,
				RequireColons: emoji.RequireColons,
				Animated:      emoji.Animated,
				APIName:       emoji.APIName(),
			})
		}
	}

	for _, lookupGuildID := range lookupGuildIDs {
		guild, _ := cache.GetSession().SessionForGuildS(guildID).State.Guild(lookupGuildID)
		if guild != nil && guild.ID != "" {
			joinedAt, _ := guild.JoinedAt.Parse()

			botPrefix := helpers.GetPrefixForServer(guild.ID)

			channels := make([]models.Rest_Channel, 0)
			sort.Slice(guild.Channels, func(i, j int) bool { return guild.Channels[i].Position < guild.Channels[j].Position })
			for _, channel := range guild.Channels {
				channelType := "text"
				switch channel.Type {
				case discordgo.ChannelTypeGuildVoice:
					channelType = "voice"
				case discordgo.ChannelTypeGuildCategory:
					channelType = "category"
				}
				channels = append(channels, models.Rest_Channel{
					ID:       channel.ID,
					ParentID: channel.ParentID,
					GuildID:  channel.GuildID,
					Name:     channel.Name,
					Type:     channelType,
					Topic:    channel.Topic,
					Position: channel.Position,
				})
			}

			eventlog.Guilds = append(eventlog.Guilds, models.Rest_Guild{
				ID:        guild.ID,
				Name:      guild.Name,
				Icon:      guild.Icon,
				OwnerID:   guild.OwnerID,
				JoinedAt:  joinedAt,
				BotPrefix: botPrefix,
				Features:  getGuildFeatures(guild.ID),
				Channels:  channels,
				Settings:  getGuildSettings(guild.ID, request.Attribute("UserID").(string)),
			})
		}
	}

	response.WriteEntity(eventlog)
}

func GetVanityInviteByName(request *restful.Request, response *restful.Response) {
	vanityName := request.PathParameter("vanity-name")
	referer := request.QueryParameter("referer")

	vanityInvite, _ := helpers.GetVanityUrlByVanityName(vanityName)
	if vanityInvite.GuildID == "" {
		response.WriteError(http.StatusNoContent, errors.New("vanity invite not found"))
		return
	}

	code, _ := helpers.GetDiscordInviteByVanityInvite(vanityInvite)
	if code == "" {
		response.WriteError(http.StatusNoContent, errors.New("unable to create invite"))
		return
	}

	go func() {
		helpers.ElasticAddVanityInviteClick(vanityInvite, referer)
	}()

	response.WriteEntity(models.Rest_VanityInvite_Invite{
		Code:    code,
		GuildID: vanityInvite.GuildID,
	})
}

func GetFileByFilehash(request *restful.Request, response *restful.Response) {
	fileHash := request.PathParameter("filehash")

	filename, filetype, data, err := helpers.RetrieveFileByHash(fileHash)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			response.WriteError(http.StatusNotFound, errors.New("file not found"))
			return
		}
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	response.WriteEntity(models.Rest_File{
		FileType: filename,
		FileName: filetype,
		Data:     base64.StdEncoding.EncodeToString(data),
	})
}

func SetGuildSettings(request *restful.Request, response *restful.Response) {
	guildID := request.PathParameter("guild-id")

	if request.Attribute("UserID").(string) != "global" && !helpers.GetIsInGuild(guildID, request.Attribute("UserID").(string)) {
		response.WriteErrorString(401, "401: Not Authorized")
		return
	}

	newSettings := new(models.Rest_Receive_SetSettings)
	err := request.ReadEntity(&newSettings)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	for _, newStringSetting := range newSettings.Strings {
		err = setGuildStringSetting(guildID, request.Attribute("UserID").(string), newStringSetting.Key, newStringSetting.Values)
		if err != nil {
			response.WriteError(http.StatusUnauthorized, err)
			return
		}
	}

	return
}

func GetAllBackgrounds(request *restful.Request, response *restful.Response) {
	var entryBucket []models.ProfileBackgroundEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.ProfileBackgroundsTable).Find(nil)).All(&entryBucket)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	backgrounds := make([]models.Rest_Background, 0)
	for _, entry := range entryBucket {
		link := entry.URL
		if link == "" {
			link, err = helpers.GetFileLink(entry.ObjectName)
			if err != nil {
				helpers.RelaxLog(err)
				continue
			}
		}

		backgrounds = append(backgrounds, models.Rest_Background{
			Name: entry.Name,
			URL:  link,
			Tags: entry.Tags,
		})
	}

	response.WriteEntity(backgrounds)
	return
}

func Ping(_ *restful.Request, response *restful.Response) {
	response.Write([]byte("pong"))
	return
}
