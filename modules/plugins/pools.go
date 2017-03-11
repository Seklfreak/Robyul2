package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
)

// Pool command usage:
//  pool create <TITLE> /// <FIELD_1>, <FIELD_2>, <FIELD3_>  => creates a pool
//  pool remove <POOL_ID>                                    => removes a pool
//  pool <POOL_ID> add field <FIELD>                         => adds a field
//  pool <POOL_ID> remove field <FIELD_ID>                   => removes a field
//  pool <POOL_ID> open                                      => opens a pool
//  pool <POOL_ID> close                                     => closes a pool
//  pool <POOL_ID>                                           => displays info about the pool
//  pool list                                                => lists all stored pools for the guild
type Pool struct {}

//Commands func
func (p *Pool) Commands() []string {
    return []string {
        "pool",
    }
}

// Init func
func (p *Pool) Init(s *discordgo.Session) {}

func (p *Pool) Action(command, content string, msg *discordgo.Message, session *discordgo.Session) {
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

func (p *Pool) create(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Pool) remove(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Pool) addField(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Pool) removeField(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Pool) open(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Pool) close(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Pool) info(content string, msg *discordgo.Message, session *discordgo.Session) {}

func (p *Pool) list(content string, msg *discordgo.Message, session *discordgo.Session) {}
