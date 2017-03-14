package migrations

func m9_create_table_bias() {
    CreateTableIfNotExists("bias")
}
