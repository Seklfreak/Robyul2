package triggers

import "git.lukas.moe/sn0w/Karen/helpers"

type EightBall struct {}

func (e EightBall) Triggers() []string {
    return []string{
        "8ball",
        "8",
    }
}

func (e EightBall) Response() string {
    return ":8ball: " + helpers.GetText("triggers.8ball")
}
