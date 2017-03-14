package migrations

func m13_create_table_notifications_ignored_channels() {
    CreateTableIfNotExists("notifications_ignored_channels")
}
