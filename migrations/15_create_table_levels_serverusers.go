package migrations

func m15_create_table_levels_serverusers() {
    CreateTableIfNotExists("levels_serverusers")
}
