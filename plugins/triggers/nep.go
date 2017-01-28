package triggers

import "git.lukas.moe/sn0w/Karen/helpers"

type Nep struct {}

func (n Nep) Triggers() []string {
    return []string{
        "nep",
        "nepgear",
        "neptune",
    }
}

func (n Nep) Response() string {
    return helpers.GetText("triggers.nep.text") + "\n" + helpers.GetText("triggers.nep.link")
}
