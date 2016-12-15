package migrations

func M1_create_table_guild_config() {
    CreateTableIfNotExists("guild_configs")
}