package triggers

type Donate struct{}

func (d *Donate) Triggers() []string {
    return []string{
        "donate",
    }
}

func (d *Donate) Response(trigger string, content string) string {
    return "Thank you so much :3 \n https://www.patreon.com/sn0w"
}
