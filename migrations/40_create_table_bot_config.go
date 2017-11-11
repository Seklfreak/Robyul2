package migrations

func m40_create_table_bot_config() {
	CreateTableIfNotExists("bot_config")
}
