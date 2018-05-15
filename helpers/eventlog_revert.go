package helpers

import (
	"strconv"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

func CanRevert(item models.ElasticEventlog) bool {
	switch item.ActionType {
	case models.EventlogTypeChannelUpdate:
		if len(item.Changes) > 0 {
			return true
		}
	}

	return false
}

func Revert(item models.ElasticEventlog) (err error) {
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
		return err
	}

	return errors.New("eventlog action type not supported")
}
