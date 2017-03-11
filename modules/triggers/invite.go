package triggers

type Invite struct{}

func (i *Invite) Triggers() []string {
    return []string{
        "invite",
        "inv",
    }
}

func (i *Invite) Response(trigger string, content string) string {
    return "Woah thanks :heart_eyes:\nI'm still beta but you can apply for early-access here: <https://karen.vc/invite>"
}
