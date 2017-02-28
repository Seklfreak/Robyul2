package migrations

func m8_create_table_lastfm() {
	CreateTableIfNotExists("lastfm")
}
