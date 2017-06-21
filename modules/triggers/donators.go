package triggers

type Donators struct{}

func (d *Donators) Triggers() []string {
    return []string{
        "donators",
        "donations",
        "donate",
        "supporters",
        "support",
        "patreon",
        "patreons",
    }
}

func (d *Donators) Response(trigger string, content string) string {
    return "<:robyulblush:327206930437373952> **These awesome people support me:**\nKakkela 💕\nThank you so much!\n_You want to be in this list? <https://www.patreon.com/sekl>!_"
}
