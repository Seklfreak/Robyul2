package migrations

func m2_create_table_reminders() {
    CreateTableIfNotExists("reminders")
}
