package migrations

func m16_create_table_galleries() {
    CreateTableIfNotExists("galleries")
}
