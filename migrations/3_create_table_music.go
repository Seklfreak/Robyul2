package migrations

func m3_create_table_music() {
    CreateTableIfNotExists("music")
}
