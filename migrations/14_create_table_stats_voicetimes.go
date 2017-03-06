package migrations

func m14_create_table_stats_voicetimes() {
    CreateTableIfNotExists("stats_voicetimes")
}
