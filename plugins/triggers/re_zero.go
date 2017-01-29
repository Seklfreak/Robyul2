package triggers

import "git.lukas.moe/sn0w/Karen/helpers"

type ReZero struct {}

func (r ReZero) Triggers() []string {
    return []string{
        "rem",
        "re:zero",
        "rezero",
    }
}

func (r ReZero) Response() string {
    return helpers.GetText("triggers.re_zero.link")
}
