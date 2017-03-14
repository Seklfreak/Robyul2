package migrations

func m10_create_table_guild_announcements() {
    CreateTableIfNotExists("guild_announcements")
}
