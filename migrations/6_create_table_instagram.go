package migrations

func m6_create_table_instagram() {
	CreateTableIfNotExists("instagram")
}
