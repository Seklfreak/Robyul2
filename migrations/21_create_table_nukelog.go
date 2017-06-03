package migrations

func m21_create_table_nukelog() {
    CreateTableIfNotExists("nukelog")
}
