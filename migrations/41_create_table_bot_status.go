package migrations

func m41_create_table_bot_status() {
	CreateTableIfNotExists("bot_status")
}
