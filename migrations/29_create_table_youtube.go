package migrations

func m29_create_table_youtube() {
	CreateTableIfNotExists("youtube")
}
