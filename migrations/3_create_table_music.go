package migrations

func M3_create_table_music() {
    CreateTableIfNotExists("music")
}
