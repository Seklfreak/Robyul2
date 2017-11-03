package migrations

func m39_create_table_donators() {
	CreateTableIfNotExists("donators")
}
