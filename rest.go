package main

import (
    "github.com/emicklei/go-restful"
    "github.com/Seklfreak/Robyul2/cache"
    "time"
    "github.com/getsentry/raven-go"
    "fmt"
    "github.com/Seklfreak/Robyul2/helpers"
    "net/http"
    "github.com/pkg/errors"
    "github.com/bwmarrin/discordgo"
)

type Rest_Guild struct {
    ID string
    Name string
    Icon string
    OwnerID string
    JoinedAt time.Time
}

type Rest_User struct {
    ID string
    Username string
    AvatarHash string
    Discriminator string
    Bot bool
}

type Rest_Member struct {
    GuildID string
    JoinedAt time.Time
    Nick string
    Roles []string
}

type Rest_Is_Member struct {
    IsMember bool
}

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

    return services
}

func GetAllBotGuilds(request *restful.Request, response *restful.Response) {
    allGuilds := cache.GetSession().State.Guilds

    returnGuilds := make([]Rest_Guild, 0)
    for _, guild := range allGuilds {
        joinedAt, err := guild.JoinedAt.Parse()
        if err != nil {
            joinedAt = time.Now()
            raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
        }

        returnGuilds = append(returnGuilds, Rest_Guild{
            ID: guild.ID,
            Name: guild.Name,
            Icon: guild.Icon,
            OwnerID: guild.OwnerID,
            JoinedAt: joinedAt,
        })
    }

    response.WriteEntity(returnGuilds)
}

func FindUser(request *restful.Request, response *restful.Response) {
    userID := request.PathParameter("user-id")

    user, _ := helpers.GetUser(userID)
    if user != nil && user.ID != "" {
        returnUser := &Rest_User{
            ID: user.ID,
            Username: user.Username,
            AvatarHash: user.Avatar,
            Discriminator: user.Discriminator,
            Bot: user.Bot,
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

        returnUser := &Rest_Member{
            GuildID: member.GuildID,
            JoinedAt: joinedAt,
            Nick: member.Nick,
            Roles: member.Roles,
        }

        response.WriteEntity(returnUser)
    } else {
        response.WriteError(http.StatusNotFound, errors.New("Member not found."))
    }
}


func IsMember(request *restful.Request, response *restful.Response) {
    guildID := request.PathParameter("guild-id")
    userID := request.PathParameter("user-id")

    isInGuild, _ := helpers.GetIsInGuild(guildID, userID)
    if isInGuild == true {
        response.WriteEntity(&Rest_Is_Member{
            IsMember: true,
        })
    } else {
        response.WriteEntity(&Rest_Is_Member{
            IsMember: false,
        })
    }
}
