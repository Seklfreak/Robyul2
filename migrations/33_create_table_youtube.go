package migrations

func m33_create_table_youtube() {
	CreateTableIfNotExists("youtube")
}
