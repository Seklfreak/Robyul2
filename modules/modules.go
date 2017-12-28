package modules

import (
	"github.com/Seklfreak/Robyul2/modules/plugins"
	"github.com/Seklfreak/Robyul2/modules/plugins/youtube"
	//"github.com/Seklfreak/Robyul2/modules/triggers"
	"github.com/Seklfreak/Robyul2/modules/plugins/google"
	"github.com/Seklfreak/Robyul2/modules/triggers"
)

var (
	pluginCache         map[string]*Plugin
	triggerCache        map[string]*TriggerPlugin
	extendedPluginCache map[string]*ExtendedPlugin

	PluginList = []Plugin{
		&plugins.About{},
		&plugins.Stats{},
		&plugins.Uptime{},
		&plugins.Translator{},
		&plugins.UrbanDict{},
		&plugins.Weather{},
		&plugins.VLive{},
		&plugins.Instagram{},
		&plugins.Facebook{},
		&plugins.WolframAlpha{},
		&plugins.LastFm{},
		&plugins.Twitch{},
		&plugins.Charts{},
		&plugins.Choice{},
		&plugins.Osu{},
		&plugins.Reminders{},
		&plugins.Ratelimit{},
		&plugins.Gfycat{},
		&plugins.RandomPictures{},
		&youtube.Handler{},
		&plugins.Spoiler{},
		&plugins.RandomCat{},
		&plugins.RPS{},
		&plugins.Nuke{},
		&plugins.Dig{},
		&plugins.Streamable{},
		&plugins.Troublemaker{},
		&plugins.Lyrics{},
		&plugins.Friend{},
		&plugins.Names{},
		&plugins.Reddit{},
		&plugins.Color{},
		&plugins.Dog{},
		&plugins.Debug{},
		&plugins.Donators{},
		&plugins.Ping{},
		&google.Handler{},
		&plugins.BotStatus{},
		&plugins.VanityInvite{},
		&plugins.DiscordMoney{},
		&plugins.Whois{},
		&plugins.Isup{},
	}

	PluginExtendedList = []ExtendedPlugin{
		&plugins.Bias{},
		&plugins.GuildAnnouncements{},
		&plugins.Notifications{},
		&plugins.Levels{},
		&plugins.Gallery{},
		&plugins.Mirror{},
		&plugins.CustomCommands{},
		&plugins.ReactionPolls{},
		&plugins.Mod{},
		&plugins.AutoRoles{},
		&plugins.Starboard{},
		&plugins.Autoleaver{},
		&plugins.Persistency{},
		&plugins.DM{},
		&plugins.Twitter{},
	}

	TriggerPluginList = []TriggerPlugin{
		&triggers.EightBall{},
	}
)
