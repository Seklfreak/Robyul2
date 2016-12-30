package helpers

var BOT_ADMINS = []string{
    "157834823594016768",
}

func IsBotAdmin(id string) bool {
    for _, s := range BOT_ADMINS {
        if s == id {
            return true
        }
    }

    return false
}
