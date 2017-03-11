package models

import (
    "time"
    "git.lukas.moe/sn0w/Karen/helpers"
    "fmt"
    "github.com/bwmarrin/discordgo"
)

// Pool struct
type Pool struct {
    // ID is the ID of the msg that created the current Pool
    ID string `rethink:"id"`
    // Title of the Pool
    Title string `rethink:"title"`
    // Fields of the pool
    Fields []*PoolField `rethink:"data"`
    // Active shows the current state for the pool
    Active bool `rethink:"active"`
    // The time when the pool was created
    CreatedAt time.Time `rethink:"created_at"`
    // The time when the pool state changed to inactive
    ClosedAt time.Time `rethink:"closed_at"`
    // CreatedBy contains the user ID that created the Pool
    // this user
    CreatedBy string `rethink:"created_by"`
}

// PoolField is a field for a Pool
type PoolField struct {
    // TODO: Choose how this id will be handled/generated
    ID    string `rethink:"id"`
    Title string `rethink:"name"`
    Votes int64  `rehink:"votes"`
}

// NewPool creates a pool for the guild
func NewPool(title, guild string, msg *discordgo.Message, fields []*PoolField) {
    settings := helpers.GuildSettingsGetCached(guild)
    settings.Pools = append(settings.Pools, &Pool{
        ID: msg.ID,
        Title: title,
        Fields: fields,
        Active: true,
        CreatedAt: time.Now(),
        CreatedBy: msg.Author.ID,
    })
    helpers.GuildSettingsSet(guild, settings)
}

// RemovePool removes a pool from the guild
func RemovePool(guild, poolID string) {
    settings := helpers.GuildSettingsGetCached(guild)
    pools := []*Pool{}
    for _, p := range settings.Pools {
        if p.ID == poolID {
            continue
        }
        pools = append(pools, p)
    }
    settings.Pools = pools
    helpers.GuildSettingsSet(guild, settings)
}

// GetPool returns a *Pool
func GetPool(guild, msgID string) (*Pool, error) {
    settings := helpers.GuildSettingsGetCached(guild)
    for _, p := range settings.Pools {
        if p.ID == msgID {
            return p, nil
        }
    }
    return nil, fmt.Errorf("Pool not found")
}

// GetPoolAndSave allows to save changes on the returned *Pool
func GetPoolAndSave(guild, poolID string) (*Pool, func(), error) {
    settings := helpers.GuildSettingsGetCached(guild)
    for _, p := range settings.Pools {
        if p.ID == poolID {
            return p, func() {
                helpers.GuildSettingsSet(guild, settings)
            }, nil
        }
    }
    return nil, func() {}, fmt.Errorf("Pool not found")
}

// AddPoolField adds the new field
func AddPoolField(guild, poolID, title string) {
    settings := helpers.GuildSettingsGetCached(guild)
    for _, p := range settings.Pools {
        if p.ID == poolID {
            p.Fields = append(p.Fields, &PoolField{
                ID:    "", // TODO: find some kind of ID
                Title: title,
                Votes: 0,
            })
            break
        }
    }
    helpers.GuildSettingsSet(guild, settings)
}

// RemovePoolField removes the pool field with ID = fieldID
func RemovePoolField(guild, poolID, fieldID string) {
    settings := helpers.GuildSettingsGetCached(guild)
    for _, p := range settings.Pools {
        if p.ID == poolID {
            fields := []*PoolField{}
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
    helpers.GuildSettingsSet(guild, settings)
}
