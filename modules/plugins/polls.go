package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
)

// Poll command usage:
//  poll create <TITLE> /// <FIELD_1>, <FIELD_2>, <FIELD3_>  => creates a poll
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

func (p *Poll) create(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) remove(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) addField(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) removeField(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) open(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) close(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) info(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Poll) list(content string, msg *discordgo.Message, session *discordgo.Session) {}
