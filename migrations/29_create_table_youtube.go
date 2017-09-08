package migrations

func m27_create_table_youtube() {
	CreateTableIfNotExists("youtube")
}
