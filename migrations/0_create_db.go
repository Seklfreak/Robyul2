package migrations

import "github.com/sn0w/Karen/utils"

func m0_create_db() {
    CreateDBIfNotExists(
        utils.GetConfig().Path("rethink.db").Data().(string),
    )
}
