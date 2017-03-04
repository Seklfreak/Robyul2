package triggers

type Shrug struct{}

func (s *Shrug) Triggers() []string {
    return []string{
        "shrug",
    }
}

func (s *Shrug) Response(trigger string, content string) string {
    return "¯\\_(ツ)_/¯"
}
