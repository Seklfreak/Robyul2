package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "git.lukas.moe/sn0w/Karen/models"
    "git.lukas.moe/sn0w/Karen/helpers"
    "git.lukas.moe/sn0w/Karen/metrics"
    "fmt"
)

// Poll command usage:
//  poll create <TITLE> /// <FIELD_1>, <FIELD_2>, <FIELD3_>, => creates a poll
//  poll remove <POOL_ID>                                    => removes a poll
//  poll <POOL_ID> add field <FIELD>                         => adds a field
//  poll <POOL_ID> remove field <FIELD_ID>                   => removes a field
//  poll <POOL_ID> open                                      => opens a poll
//  poll <POOL_ID> close                                     => closes a poll
//  poll <POOL_ID>                                           => displays info about the poll
//  poll list                                                => lists all stored polls for the guild
type Poll struct {}

//Commands func
func (p *Poll) Commands() []string {
    return []string {
        "poll", 
    }
}

// Init func
func (p *Poll) Init(s *discordgo.Session) {}

// Action func 
func (p *Poll) Action(command, content string, msg *discordgo.Message, session *discordgo.Session) {
    msgSplit := strings.Fields(content)
    if len(msgSplit) < 1 {
        return
    }
    switch msgSplit[0] {
    case "create":
        p.create(content, msg, session)
    case "remove":
        p.remove(content, msg, session)
    case "list":
        p.list(content, msg, session)
    default:
        switch msgSplit[1] {
        case "add":
            if msgSplit[2] == "field" {
                p.addField(content, msg, session)
            }
        case "remove":
            if msgSplit[2] == "field" {
                p.removeField(content, msg, session)
            }
        case "open":
            p.open(content, msg, session)
        case "close":
            p.close(content, msg, session)
        default:
            p.info(content, msg, session)
        }
    }
}

func (p *Poll) create(content string, msg *discordgo.Message, session *discordgo.Session) {
    msgSplit := strings.Fields(content)
    msgSplit = msgSplit[1:]
    title := []string{}
    separator := 0
    for i, v := range msgSplit {
        if v != "///" {
            title = append(title, v)
        }
        if v == "///" {
            separator = i
            break
        }
    }
    joinedTitle := strings.Join(title, " ")
    rest := msgSplit[separator+1:]
    fields := []models.PollField{}
    temp := ""
    generator := helpers.NewPollFieldGenerator()
    for _, v := range rest {
        if strings.HasSuffix(v, ",") {
            if temp != "" {
                temp += v[:len(v)-1]
                fields = append(fields, generator(temp))
                temp = ""
            } else {
                fields = append(fields, generator(v[:len(v)-1]))
            }
        } else {
            temp += v + " "
        }
    }
    if len(fields) > 10 {
        fields = fields[:10]
    }
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }

    mfields := []*discordgo.MessageEmbedField{}
    for _, field := range fields {
        mfields = append(mfields, &discordgo.MessageEmbedField{
            Name: fmt.Sprintf("%v) %v", field.ID, field.Title),
            Value: fmt.Sprintf("%v votes", field.Votes),
        })
    }

    embed := &discordgo.MessageEmbed{
        Title:  joinedTitle,
        Fields: mfields,
        Color:  0x0FADED,
    }
    

    m, err := session.ChannelMessageSendEmbed(msg.ChannelID, embed)
    if err != nil {
        return
    }

    for _, field := range fields {
        session.MessageReactionAdd(m.ChannelID, m.ID, fmt.Sprintf(":%v:", helpers.HumanizeNumber(field.ID)))
    }

    embed.Footer = &discordgo.MessageEmbedFooter {
        Text: fmt.Sprintf("Poll ID: %v", m.ID),
    }

    session.ChannelMessageEditEmbed(m.ChannelID, m.ID, embed)

    helpers.NewPoll(joinedTitle, channel.GuildID, m.ID, msg.Author.ID, fields...)
    metrics.PollsCreated.Add(1)
}

func (p *Poll) remove(content string, msg *discordgo.Message, session *discordgo.Session) {
    msgSplit := strings.Fields(content)
    if len(msgSplit) < 2 {
        return
    }
    pollID := msgSplit[1]
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    helpers.RemovePoll(channel.GuildID, pollID)
}

func (p *Poll) addField(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) removeField(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) open(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) close(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) info(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) list(content string, msg *discordgo.Message, session *discordgo.Session) {}
