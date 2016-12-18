package migrations

func M2_create_table_reminders() {
    CreateTableIfNotExists("reminders")
}
