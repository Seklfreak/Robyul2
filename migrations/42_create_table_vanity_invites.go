package migrations

func m42_create_table_vanity_invites() {
	CreateTableIfNotExists("vanity_invites")
}
