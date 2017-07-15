package migrations

import (
    rethink "github.com/gorethink/gorethink"
    "github.com/Seklfreak/Robyul2/helpers"
)

func m14_create_table_stats_voicetimes() {
    CreateTableIfNotExists("stats_voicetimes")

    rethink.Table("stats_voicetimes").IndexCreate("guildid").Run(helpers.GetDB())
    rethink.Table("stats_voicetimes").IndexCreate("userid").Run(helpers.GetDB())
}
