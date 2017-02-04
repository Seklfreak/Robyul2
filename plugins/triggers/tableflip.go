package triggers

type TableFlip struct{}

func (t TableFlip) Triggers() []string {
    return []string{
        "tableflip",
        "flip",
        "tbf",
    }
}

func (t TableFlip) Response(trigger string, content string) string {
    return "(╯°□°）╯︵ ┻━┻"
}
