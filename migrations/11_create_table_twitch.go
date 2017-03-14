package migrations

func m11_create_table_twitch() {
    CreateTableIfNotExists("twitch")
}
