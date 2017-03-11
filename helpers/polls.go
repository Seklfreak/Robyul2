package helpers

import (
    "time"
    "github.com/bwmarrin/discordgo"
    "git.lukas.moe/sn0w/Karen/models"
    "errors"
)

// NewPoll creates a pool for the guild
func NewPoll(title, guild string, msg *discordgo.Message, fields ...models.PollField) {
    settings := GuildSettingsGetCached(guild)
    settings.Polls = append(settings.Polls, models.Poll{
        ID:        msg.ID,
        Title:     title,
        Fields:    fields,
        Open:      true,
        CreatedAt: time.Now(),
        CreatedBy: msg.Author.ID,
    })
    GuildSettingsSet(guild, settings)
}

//NewPollField returns a new PollField
func NewPollField(title string) models.PollField {
    return models.PollField{
        ID: "", // TODO: generate some kind of id?
        Title: title,
        Votes: 0,
    }
}

// RemovePoll removes a pool from the guild
func RemovePoll(guild, pollID string) {
    settings := GuildSettingsGetCached(guild)
    polls := []models.Poll{}
    for _, p := range settings.Polls {
        if p.ID == pollID {
            continue
        }
        polls = append(polls, p)
    }
    settings.Polls = polls
    GuildSettingsSet(guild, settings)
}

// GetPoll returns a Poll
func GetPoll(guild, pollID string) (models.Poll, error) {
    settings := GuildSettingsGetCached(guild)
    for _, p := range settings.Polls {
        if p.ID == pollID {
            return p, nil
        }
    }
    return models.Poll{}, errors.New("Poll not found")
}

// UpdatePoll updates a poll
func UpdatePoll(guild string, poll models.Poll) {
    settings := GuildSettingsGetCached(guild)
    for i, p := range settings.Polls {
        if p.ID == poll.ID {
            settings.Polls[i] = poll
            break
        }
    }
    GuildSettingsSet(guild, settings)
}

// AddPollField adds the new field
func AddPollField(guild, pollID, title string) {
    settings := GuildSettingsGetCached(guild)
    for _, p := range settings.Polls {
        if p.ID == pollID {
            p.Fields = append(p.Fields, models.PollField{
                ID:    "", // TODO: find some kind of ID
                Title: title,
                Votes: 0,
            })
            break
        }
    }
    GuildSettingsSet(guild, settings)
}

// RemovePollField removes the pool field with ID = fieldID
func RemovePollField(guild, pollID, fieldID string) {
    settings := GuildSettingsGetCached(guild)
    for _, p := range settings.Polls {
        if p.ID == pollID {
            fields := []models.PollField{}
            for _, pf := range p.Fields {
                if pf.ID == fieldID {
                    continue
                }
                fields = append(fields, pf)
            }
            p.Fields = fields
            break
        }
    }
    GuildSettingsSet(guild, settings)
}

// CanVotePoll returns true if the user didn't vote yet
func CanVotePoll(guild, pollID, userID string) bool {
    settings := GuildSettingsGetCached(guild)
    for _, p := range settings.Polls {
        if p.ID == pollID {
            for _, participant := range p.Participants {
                if participant.ID == userID {
                    return false
                }
            }
            return true
        }
    }
    return false
}

// PollTotalVotes returns the total votes for a poll
func PollTotalVotes(guild, pollID string) int64 {
    settings := GuildSettingsGetCached(guild)
    for _, p := range settings.Polls {
        if p.ID == pollID {
            return p.TotalVotes
        }
    }
    return 0
}

// PollTotalParticipants returns the total participants for a poll
func PollTotalParticipants(guild, pollID string) int64 {
    settings := GuildSettingsGetCached(guild)
    for _, p := range settings.Polls {
        if p.ID == pollID {
            return p.TotalParticipants
        }
    }
    return 0
}
