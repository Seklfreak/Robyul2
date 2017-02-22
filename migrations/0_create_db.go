package migrations

import "github.com/Seklfreak/Robyul2/helpers"

func m0_create_db() {
	CreateDBIfNotExists(
		helpers.GetConfig().Path("rethink.db").Data().(string),
	)
}
