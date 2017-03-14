package migrations

func m12_create_table_notifications() {
    CreateTableIfNotExists("notifications")
}
