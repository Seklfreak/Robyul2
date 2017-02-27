package triggers

type Lenny struct{}

func (l *Lenny) Triggers() []string {
    return []string{
        "lenny",
    }
}

func (l *Lenny) Response(trigger string, content string) string {
    return "( ͡° ͜ʖ ͡°)"
}
