package migrations

func m18_create_table_randompictures_sources() {
	CreateTableIfNotExists("randompictures_sources")
}
