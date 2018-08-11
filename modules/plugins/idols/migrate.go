package idols

import (
	"fmt"

	"github.com/Seklfreak/Robyul2/cache"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

// migrateIdols will migrate old idol records where each image was saved
// seporatly to the new table
func migrateIdols(msg *discordgo.Message, content string) {

	// check if the new table already exists and has records. if it does block the migration again
	count, err := helpers.GetMDb().C(models.IdolTable.String()).Count()
	if err != nil {
		cache.GetLogger().Errorln(err.Error())
		return
	}
	if count > 0 {
		helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Migration has already been run. If something went wrong please drop table **%s** and run this again.", models.IdolTable))
		return
	}

	// refresh the idols from mongo again to make sure the data is up to date
	helpers.SendMessage(msg.ChannelID, "Refreshing Idols from old table....")
	refreshIdolsFromOld(true)
	helpers.SendMessage(msg.ChannelID, "Idols refreshed")
	helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Idols Found: %d", len(GetAllIdols())))

	// convert current in memory idol records to the new tables idol entries
	helpers.SendMessage(msg.ChannelID, "Migrating idols to new table...")
	for _, idol := range GetAllIdols() {
		newIdolEntry := models.IdolEntry{
			ID:        "",
			Name:      idol.Name,
			GroupName: idol.GroupName,
			Gender:    idol.Gender,
		}

		// convert current idol images to new table images entries
		for _, img := range idol.Images {
			newImageEntry := models.IdolImageEntry{
				HashString: img.HashString,
				ObjectName: img.ObjectName,
			}
			newIdolEntry.Images = append(newIdolEntry.Images, newImageEntry)
		}
		if len(newIdolEntry.Images) == 0 {
			helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Could not migrate %s %s due to missing images", newIdolEntry.GroupName, newIdolEntry.Name))
			continue
		}
		_, err := helpers.MDbInsert(models.IdolTable, newIdolEntry)
		if err != nil {
			cache.GetLogger().Errorln(err.Error())
		}
	}

	refreshIdols(true)

	helpers.SendMessage(msg.ChannelID, "Migration done.")
}
