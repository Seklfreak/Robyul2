package triggers

type Invite struct{}

func (i Invite) Triggers() []string {
    return []string{
        "invite",
        "inv",
    }
}

func (i Invite) Response() string {
    return "Woah thanks :heart_eyes:\nI'm still beta but you can apply for early-access here: <https://meetkaren.xyz/invite>"
}
