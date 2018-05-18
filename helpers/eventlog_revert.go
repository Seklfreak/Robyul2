package helpers

import (
	"strconv"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
)

func CanRevert(item models.ElasticEventlog) bool {
	if item.Reverted {
		return false
	}

	switch item.ActionType {
	case models.EventlogTypeChannelUpdate:
		if len(item.Changes) > 0 {
			return true
		}
	}

	return false
}

func Revert(eventlogID, userID string, item models.ElasticEventlog) (err error) {
	switch item.ActionType {
	case models.EventlogTypeChannelUpdate:
		channel, err := GetChannel(item.TargetID)
		if err != nil {
			return err
		}

		channelEdit := &discordgo.ChannelEdit{ // restore ints because go
			Position: channel.Position,
			Bitrate:  channel.Bitrate,
		}
		for _, change := range item.Changes {
			switch change.Key {
			case "channel_name":
				channelEdit.Name = change.OldValue
			case "channel_topic":
				channelEdit.Topic = change.OldValue
			case "channel_nsfw":
				channelEdit.NSFW = GetStringAsBool(change.OldValue)
			case "channel_bitrate":
				newBitrate, err := strconv.Atoi(change.OldValue)
				if err == nil {
					channelEdit.Bitrate = newBitrate
				}
			case "channel_parentid":
				channelEdit.ParentID = change.OldValue
			}
		}

		cache.GetSession().ChannelEditComplex(item.TargetID, channelEdit)
		if err != nil {
			return err
		}

		return logRevert(channel.GuildID, userID, eventlogID)
	}

	return errors.New("eventlog action type not supported")
}

func logRevert(guildID, userID, eventlogID string) error {
	// add new eventlog entry for revert
	_, err := EventlogLog(time.Now(), guildID, eventlogID,
		models.EventlogTargetTypeRobyulEventlogItem, userID,
		models.EventlogTypeRobyulActionRevert, "",
		nil,
		nil,
		false,
	)
	if err != nil {
		return err
	}

	// get issuer user
	user, err := GetUserWithoutAPI(userID)
	if err != nil {
		user = new(discordgo.User)
		user.ID = userID
		user.Username = "N/A"
		user.Discriminator = "N/A"
	}

	// add option to reverted action with information
	err = EventlogLogUpdate(
		eventlogID,
		"",
		[]models.ElasticEventlogOption{{
			Key:   "reverted_by_userid",
			Value: user.ID,
			Type:  models.EventlogTargetTypeUser,
		}},
		nil,
		"",
		false,
		true,
	)
	spew.Dump(err)
	return err
}
