package robyulstate

import (
	"sync"

	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"github.com/getsentry/raven-go"
	"github.com/olivere/elastic"
)

type Robyulstate struct {
	sync.RWMutex

	guildMap map[string]*discordgo.Guild

	Logger func(msgL, caller int, format string, a ...interface{})
}

func NewState() *Robyulstate {
	return &Robyulstate{
		guildMap: make(map[string]*discordgo.Guild),
	}
}

func (s *Robyulstate) OnInterface(_ *discordgo.Session, i interface{}) {
	defer func() {
		err := recover()
		if err != nil {
			s.Logger(discordgo.LogError, 0, fmt.Sprintf("Recover: %s", spew.Sdump(err)))

			if errE, ok := err.(*elastic.Error); ok {
				raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{
					"Type":     errE.Details.Type,
					"Reason":   errE.Details.Reason,
					"Index":    errE.Details.Index,
					"CausedBy": spew.Sdump(errE.Details.CausedBy),
				})
			} else {
				raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{})
			}
		}
	}()

	if s == nil {
		s.Logger(discordgo.LogError, 0, discordgo.ErrNilState.Error())
		return
	}

	var err error

	//fmt.Println("received event:", reflect.TypeOf(i))

	switch t := i.(type) {
	case *discordgo.GuildCreate:
		err = s.GuildAdd(t.Guild)
	case *discordgo.GuildUpdate:
		err = s.GuildUpdate(t.Guild)
	case *discordgo.GuildDelete:
		err = s.GuildRemove(t.Guild)
	case *discordgo.GuildEmojisUpdate:
		err = s.EmojisUpdate(t.GuildID, t.Emojis)
	case *discordgo.ChannelUpdate:
		err = s.ChannelUpdate(t.Channel)
		/*
			case *GuildMemberUpdate:
				if s.TrackMembers {
					err = s.MemberAdd(t.Member)
				}
			case *GuildRoleUpdate:
				if s.TrackRoles {
					err = s.RoleAdd(t.GuildID, t.Role)
				}
			case *VoiceStateUpdate:
				if s.TrackVoice {
					err = s.voiceStateUpdate(t)
				}
			case *PresenceUpdate:
				if s.TrackPresences {
					s.PresenceAdd(t.GuildID, &t.Presence)
				}
				if s.TrackMembers {
					if t.Status == StatusOffline {
						return
					}

					var m *Member
					m, err = s.Member(t.GuildID, t.User.ID)

					if err != nil {
						// Member not found; this is a user coming online
						m = &Member{
							GuildID: t.GuildID,
							Nick:    t.Nick,
							User:    t.User,
							Roles:   t.Roles,
						}

					} else {

						if t.Nick != "" {
							m.Nick = t.Nick
						}

						if t.User.Username != "" {
							m.User.Username = t.User.Username
						}

						// PresenceUpdates always contain a list of roles, so there's no need to check for an empty list here
						m.Roles = t.Roles

					}

					err = s.MemberAdd(m)
				}
		*/

	}

	if err != nil {
		s.Logger(discordgo.LogError, 0, err.Error())
	}

	return
}

func (s *Robyulstate) GuildAdd(guild *discordgo.Guild) error {
	if s == nil {
		return discordgo.ErrNilState
	}

	s.Lock()
	defer s.Unlock()

	guildCopy := new(discordgo.Guild)
	*guildCopy = *guild

	guildCopy.Roles = make([]*discordgo.Role, len(guild.Roles))
	copy(guildCopy.Roles, guild.Roles)

	guildCopy.Emojis = make([]*discordgo.Emoji, len(guild.Emojis))
	copy(guildCopy.Emojis, guild.Emojis)

	guildCopy.Members = make([]*discordgo.Member, len(guild.Members))
	copy(guildCopy.Members, guild.Members)

	guildCopy.Presences = make([]*discordgo.Presence, len(guild.Presences))
	copy(guildCopy.Presences, guild.Presences)

	guildCopy.Channels = make([]*discordgo.Channel, len(guild.Channels))
	for i, guildChannel := range guild.Channels {
		guildCopy.Channels[i] = new(discordgo.Channel)
		*guildCopy.Channels[i] = *guildChannel
	}

	guildCopy.VoiceStates = make([]*discordgo.VoiceState, len(guild.VoiceStates))
	copy(guildCopy.VoiceStates, guild.VoiceStates)

	s.guildMap[guild.ID] = guildCopy

	return nil
}

func (s *Robyulstate) GuildUpdate(guild *discordgo.Guild) error {
	if s == nil {
		return discordgo.ErrNilState
	}

	s.Lock()
	defer s.Unlock()

	if _, ok := s.guildMap[guild.ID]; !ok {
		guildCopy := new(discordgo.Guild)
		*guildCopy = *guild
		s.guildMap[guild.ID] = guildCopy
	}

	if s.guildMap[guild.ID].Name != guild.Name ||
		s.guildMap[guild.ID].Icon != guild.Icon ||
		s.guildMap[guild.ID].Region != guild.Region ||
		s.guildMap[guild.ID].AfkChannelID != guild.AfkChannelID ||
		s.guildMap[guild.ID].EmbedChannelID != guild.EmbedChannelID ||
		s.guildMap[guild.ID].OwnerID != guild.OwnerID ||
		s.guildMap[guild.ID].Splash != guild.Splash ||
		s.guildMap[guild.ID].AfkTimeout != guild.AfkTimeout ||
		s.guildMap[guild.ID].VerificationLevel != guild.VerificationLevel ||
		s.guildMap[guild.ID].EmbedEnabled != guild.EmbedEnabled ||
		s.guildMap[guild.ID].DefaultMessageNotifications != guild.DefaultMessageNotifications {
		// guild got updated
		//fmt.Println("guild update", s.guildMap[guild.ID].Name, "to", guild.Name)
		helpers.OnEventlogGuildUpdate(guild.ID, s.guildMap[guild.ID], guild)
	}

	guildCopy := new(discordgo.Guild)
	*guildCopy = *guild

	guildCopy.Roles = make([]*discordgo.Role, len(guild.Roles))
	copy(guildCopy.Roles, guild.Roles)

	guildCopy.Emojis = make([]*discordgo.Emoji, len(guild.Emojis))
	copy(guildCopy.Emojis, guild.Emojis)

	guildCopy.Members = make([]*discordgo.Member, len(guild.Members))
	copy(guildCopy.Members, guild.Members)

	guildCopy.Presences = make([]*discordgo.Presence, len(guild.Presences))
	copy(guildCopy.Presences, guild.Presences)

	guildCopy.Channels = make([]*discordgo.Channel, len(guild.Channels))
	for i, guildChannel := range guild.Channels {
		guildCopy.Channels[i] = new(discordgo.Channel)
		*guildCopy.Channels[i] = *guildChannel
	}

	guildCopy.VoiceStates = make([]*discordgo.VoiceState, len(guild.VoiceStates))
	copy(guildCopy.VoiceStates, guild.VoiceStates)

	*s.guildMap[guild.ID] = *guildCopy

	return nil
}

func (s *Robyulstate) GuildRemove(guild *discordgo.Guild) error {
	if s == nil {
		return discordgo.ErrNilState
	}

	s.Lock()
	defer s.Unlock()

	s.guildMap[guild.ID] = nil

	return nil
}

func (s *Robyulstate) EmojisUpdate(guildID string, emojis []*discordgo.Emoji) error {
	if s == nil {
		return discordgo.ErrNilState
	}

	s.Lock()
	defer s.Unlock()

	if _, ok := s.guildMap[guildID]; !ok {
		return discordgo.ErrStateNotFound
	}

	if s.guildMap[guildID].Emojis == nil {
		s.guildMap[guildID].Emojis = make([]*discordgo.Emoji, len(emojis))
		copy(s.guildMap[guildID].Emojis, emojis)
	}

	// remove guild emoji not in emojis
	for i, oldEmoji := range s.guildMap[guildID].Emojis {
		emojiRemoved := true
		for _, newEmoji := range emojis {
			if newEmoji.ID == oldEmoji.ID {
				emojiRemoved = false
			}
		}
		if emojiRemoved {
			s.guildMap[guildID].Emojis = append(s.guildMap[guildID].Emojis[:i], s.guildMap[guildID].Emojis[i+1:]...)
			// emoji got removed
			//fmt.Println("emoji removed", oldEmoji.Name)
			helpers.OnEventlogEmojiDelete(guildID, oldEmoji)
		}
	}

	// update guild emoji
	for j, oldEmoji := range s.guildMap[guildID].Emojis {
		for i, newEmoji := range emojis {
			if oldEmoji.ID == newEmoji.ID {
				if oldEmoji.Name != newEmoji.Name ||
					oldEmoji.Animated != newEmoji.Animated ||
					oldEmoji.RequireColons != newEmoji.RequireColons ||
					oldEmoji.Managed != newEmoji.Managed {
					// emoji got updated
					//fmt.Println("emoji update", oldEmoji.Name, "to", newEmoji.Name)
					helpers.OnEventlogEmojiUpdate(guildID, oldEmoji, newEmoji)
				}
				emojis = append(emojis[:i], emojis[i+1:]...)
				s.guildMap[guildID].Emojis[j] = newEmoji
			}
		}
	}

	// add guild emoji
	for _, newEmoji := range emojis {
		s.guildMap[guildID].Emojis = append(s.guildMap[guildID].Emojis, newEmoji)
		// emoji got added
		//fmt.Println("emoji added", newEmoji.Name)
		helpers.OnEventlogEmojiCreate(guildID, newEmoji)
	}

	return nil
}

func (s *Robyulstate) ChannelUpdate(newChannel *discordgo.Channel) error {
	if s == nil {
		return discordgo.ErrNilState
	}

	s.Lock()
	defer s.Unlock()

	if _, ok := s.guildMap[newChannel.GuildID]; !ok {
		return discordgo.ErrStateNotFound
	}

	if s.guildMap[newChannel.GuildID].Channels == nil {
		s.guildMap[newChannel.GuildID].Channels = make([]*discordgo.Channel, 1)
		s.guildMap[newChannel.GuildID].Channels[1] = new(discordgo.Channel)
		*s.guildMap[newChannel.GuildID].Channels[1] = *newChannel
	}

	// update channel
	for j, oldChannel := range s.guildMap[newChannel.GuildID].Channels {
		if oldChannel.ID == newChannel.ID {
			if oldChannel.Name != newChannel.Name ||
				oldChannel.Topic != newChannel.Topic ||
				oldChannel.NSFW != newChannel.NSFW ||
				oldChannel.Position != newChannel.Position ||
				oldChannel.Bitrate != newChannel.Bitrate ||
				oldChannel.ParentID != newChannel.ParentID ||
				!helpers.ChannelOverwritesMatch(oldChannel.PermissionOverwrites, newChannel.PermissionOverwrites) {
				// channel got updated
				//fmt.Println("channel update", oldChannel.Name, "to", oldChannel.Name)
				helpers.OnEventlogChannelUpdate(newChannel.GuildID, oldChannel, newChannel)
			}
			_ = j
			s.guildMap[newChannel.GuildID].Channels[j] = newChannel
		}
	}

	return nil
}
