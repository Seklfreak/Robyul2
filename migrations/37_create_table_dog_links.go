package migrations

func m37_create_table_dog_links() {
	CreateTableIfNotExists("dog_links")
}
