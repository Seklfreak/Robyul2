package migrations

func m19_create_table_customcommands() {
    CreateTableIfNotExists("customcommands")
}
