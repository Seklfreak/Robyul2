package triggers

import "git.lukas.moe/sn0w/Karen/helpers"

type EightBall struct{}

func (e *EightBall) Triggers() []string {
    return []string{
        "8ball",
        "8",
    }
}

func (e *EightBall) Response(trigger string, content string) string {
    if len(content) < 3 {
        return helpers.GetText("triggers.8ball.ask_a_question")
    }

    return ":8ball: " + helpers.GetText("triggers.8ball")
}
