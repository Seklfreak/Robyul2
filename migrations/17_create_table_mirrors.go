package migrations

func m17_create_table_mirrors() {
    CreateTableIfNotExists("mirrors")
}
