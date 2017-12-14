package migrations

func m44_create_table_user_config() {
	CreateTableIfNotExists("user_config")
}
