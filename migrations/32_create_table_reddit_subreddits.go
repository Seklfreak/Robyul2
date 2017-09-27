package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m32_create_table_reddit_subreddits() {
	CreateTableIfNotExists("reddit_subreddits")

	rethink.Table("reddit_subreddits").IndexCreate("guild_id").Run(helpers.GetDB())
}
