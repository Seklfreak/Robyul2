package plugins

import (
    "fmt"
    "git.lukas.moe/sn0w/Karen/emojis"
    "git.lukas.moe/sn0w/Karen/helpers"
    Logger "git.lukas.moe/sn0w/Karen/logger"
    "git.lukas.moe/sn0w/Karen/metrics"
    "git.lukas.moe/sn0w/Karen/models"
    "github.com/bwmarrin/discordgo"
    "strconv"
    "strings"
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
type Poll struct{}

//Commands func
func (p *Poll) Commands() []string {
    return []string{
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
        if len(msgSplit) == 1 {
            p.info(content, msg, session)
            return
        }
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
        }
    }
}

func (p *Poll) create(content string, msg *discordgo.Message, session *discordgo.Session) {
    // Getting content fields
    msgSplit := strings.Fields(content)
    // Removing 'create' from content
    msgSplit = msgSplit[1:]
    // Title of the poll
    title := []string{}
    // The index of '///' in msgSplit
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
    // Join the title
    joinedTitle := strings.Join(title, " ")
    // The rest of the msg are the fields/options
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
                if v != "," {
                    fields = append(fields, generator(v[:len(v)-1]))
                }
            }
        } else {
            temp += v + " "
        }
    }
    // Check that there are at least 2 fields
    if len(fields) < 2 {
        session.ChannelMessageSend(msg.ChannelID, "Common... Give us options!! :rolling_eyes:")
        return
    }
    // If there are more than 10 fields, just use the first 10
    if len(fields) > 10 {
        fields = fields[:10]
    }
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    // The message embed fields
    mfields := []*discordgo.MessageEmbedField{}
    for _, field := range fields {
        mfields = append(mfields, &discordgo.MessageEmbedField{
            Name:  fmt.Sprintf("%v) %v", field.ID, field.Title),
            Value: fmt.Sprintf("%v votes", field.Votes),
        })
    }
    // The embed msg
    embed := &discordgo.MessageEmbed{
        Title:  joinedTitle,
        Fields: mfields,
        Color:  0x0FADED,
    }
    // Post poll and fields and save the returned msg to use its ID
    m, err := session.ChannelMessageSendEmbed(msg.ChannelID, embed)
    if err != nil {
        return
    }
    // Pin the msg
    err = session.ChannelMessagePin(msg.ChannelID, m.ID)
    if err != nil {
        Logger.PLUGIN.L("polls.go", err.Error())
    }
    // Set the poll ID to the new msg ID
    embed.Footer = &discordgo.MessageEmbedFooter{
        Text: fmt.Sprintf("Poll ID: %v | Status: OPEN", m.ID),
    }

    for _, field := range fields {
        emoji := emojis.From(strconv.Itoa(field.ID))
        err = session.MessageReactionAdd(m.ChannelID, m.ID, emoji)
        if err != nil {
            Logger.PLUGIN.L("polls.go", err.Error())
        }
    }

    // Persist the poll to db
    if helpers.NewPoll(joinedTitle, channel.GuildID, m.ID, m.ChannelID, msg.Author.ID, fields...) {
        // Increase our counter
        metrics.PollsCreated.Add(1)

        // 'Commit' changes :3
        session.ChannelMessageEditEmbed(m.ChannelID, m.ID, embed)
    } else {
        // Notify failure
        session.ChannelMessageSend(msg.ChannelID, "Sorry, the poll could not be created, please try again senpai :bow:")
        session.ChannelMessageDelete(m.ChannelID, m.ID)
    }
}

func (p *Poll) remove(content string, msg *discordgo.Message, session *discordgo.Session) {
    // Get content fields
    msgSplit := strings.Fields(content)
    // msgSplit must have at least 2 items
    if len(msgSplit) < 2 {
        return
    }

    pollID := msgSplit[1]
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    if helpers.RemovePoll(channel.GuildID, pollID, msg) {
        // Success
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Succesfully removed poll with ID `%s`", pollID))
        session.ChannelMessageDelete(msg.ChannelID, pollID)
    } else {
        // Failure
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Sorry, could not remove poll with ID `%v`", pollID))
    }
}

func (p *Poll) addField(content string, msg *discordgo.Message, session *discordgo.Session) {
    // Get content fields
    msgSplit := strings.Fields(content)
    // msgSplit must have at least 4 items
    if len(msgSplit) < 4 {
        return
    }
    pollID := msgSplit[0]
    title := strings.Join(msgSplit[3:], " ")
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    if helpers.AddPollField(channel.GuildID, pollID, title, msg) {
        // Success
        helpers.UpdatePollMsg(channel.GuildID, pollID)
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Correctly added field `%v`", title))
    } else {
        // Failure
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Sorry, could not add field `%v` , have in mind that polls can't have more than 10 fields!!", title))
    }
}

func (p *Poll) removeField(content string, msg *discordgo.Message, session *discordgo.Session) {
    // Get content fields
    msgSplit := strings.Fields(content)
    // msgSplit must have at least 4 items
    if len(msgSplit) < 4 {
        return
    }
    pollID := msgSplit[0]
    fieldID, err := strconv.Atoi(msgSplit[3])
    if err != nil {
        session.ChannelMessageSend(msg.ChannelID, "Wrong field ID!")
        return
    }
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    if helpers.RemovePollField(channel.GuildID, pollID, fieldID, msg) {
        // Success
        if helpers.UpdatePollMsg(channel.GuildID, pollID) {
            session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Successfuly removed field with ID `%v`", fieldID))
        }
    } else {
        // Failure
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Sorry, could not remove field with ID `%v` , have in mind that poll need at least 2 fields!!", fieldID))
    }
}

func (p *Poll) open(content string, msg *discordgo.Message, session *discordgo.Session) {
    msgSplit := strings.Fields(content)
    if len(msgSplit) < 2 {
        session.ChannelMessageSend(msg.ChannelID, "Wait.. Where's the poll ID??¿ :eyes:")
        return
    }
    pollID := msgSplit[0]
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    if helpers.OpenPoll(channel.GuildID, pollID, msg) {
        helpers.UpdatePollMsg(channel.GuildID, pollID)
        // Success
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Successfuly opened `%v`", pollID))
    } else {
        // Failure
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Could not open `%v`", pollID))
    }
}

func (p *Poll) close(content string, msg *discordgo.Message, session *discordgo.Session) {
    msgSplit := strings.Fields(content)
    if len(msgSplit) < 2 {
        session.ChannelMessageSend(msg.ChannelID, "Wait.. Where's the poll ID??¿ :eyes:")
        return
    }
    pollID := msgSplit[0]
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    if helpers.ClosePoll(channel.GuildID, pollID, msg) {
        helpers.UpdatePollMsg(channel.GuildID, pollID)
        // Success
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Successfuly closed `%v`", pollID))
    } else {
        // Failure
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Could not close `%v`", pollID))
    }
}

func (p *Poll) info(content string, msg *discordgo.Message, session *discordgo.Session) {
    msgSplit := strings.Fields(content)
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    poll, err := helpers.GetPoll(channel.GuildID, msgSplit[0])
    if err != nil {
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Wrong poll ID `%s`", msgSplit[0]))
        return
    }
    embed := helpers.GetEmbedMsgFromPoll(poll)
    session.ChannelMessageSendEmbed(msg.ChannelID, embed)
}

func (p *Poll) list(content string, msg *discordgo.Message, session *discordgo.Session) {
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        return
    }
    embed := helpers.GetListEmbedMsg(channel.GuildID)
    session.ChannelMessageSendEmbed(msg.ChannelID, embed)
}
