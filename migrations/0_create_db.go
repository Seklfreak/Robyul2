package migrations

import "git.lukas.moe/sn0w/Karen/helpers"

func m0_create_db() {
    CreateDBIfNotExists(
        helpers.GetConfig().Path("rethink.db").Data().(string),
    )
}
