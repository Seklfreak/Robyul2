package migrations

func m5_create_table_twitter() {
	CreateTableIfNotExists("twitter")
}
