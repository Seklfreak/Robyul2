package plugins

import (
	"strconv"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
)

type Uptime struct{}

func (u *Uptime) Commands() []string {
	return []string{
		"uptime",
	}
}

func (u *Uptime) Init(session *shardmanager.Manager) {

}

func (u *Uptime) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermStats) {
		return
	}

	// Get uptime
	bootTime, err := strconv.ParseInt(metrics.Uptime.String(), 10, 64)
	if err != nil {
		bootTime = 0
	}

	uptime := helpers.HumanizeDuration(time.Now().Sub(time.Unix(bootTime, 0)))

	helpers.SendMessage(msg.ChannelID, ":hourglass_flowing_sand: "+uptime)
}
