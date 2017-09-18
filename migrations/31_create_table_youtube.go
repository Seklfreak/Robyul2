package migrations

func m31_create_table_youtube() {
	CreateTableIfNotExists("youtube")
}
