package helpers

var BOT_ADMINS = []string{
    "157834823594016768", // 0xFADED#3237
    "165345731706748929", // Serraniel#8978
}

func IsBotAdmin(id string) bool {
    for _, s := range BOT_ADMINS {
        if s == id {
            return true
        }
    }

    return false
}
