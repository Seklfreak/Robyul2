package migrations

func m20_create_table_reactionpolls() {
    CreateTableIfNotExists("reactionpolls")
}
