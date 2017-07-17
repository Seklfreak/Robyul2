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
        "credits",
    }
}

func (d *Donators) Response(trigger string, content string) string {
    return "<:robyulblush:327206930437373952> **These awesome people support me:**\nKakkela 💕\nSunny 💓\nsomicidal minaiac 💞\nOokami 💖\nKeldra 💗\nTN 💝\nseulguille 💘\nSlenn 💜\nFugu ❣️\nWoori 💞\nhikari 💙\nAshton 💖\nKay 💝\nThank you so much!\n_You want to be in this list? <https://www.patreon.com/sekl>!_"
}
