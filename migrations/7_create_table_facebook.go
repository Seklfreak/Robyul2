package migrations

func m7_create_table_facebook() {
	CreateTableIfNotExists("facebook")
}
