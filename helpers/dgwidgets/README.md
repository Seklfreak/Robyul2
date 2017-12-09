# dg-widgets
Make widgets with embeds and reactions

![img](https://i.imgur.com/viJc9Cm.gif)

# Example usage
```go
func (s *discordgo.Session, m *discordgo.Message) {
	p := dgwidgets.NewPaginator(s, m.ChannelID)

	// Add embed pages to paginator
	p.Add(&discordgo.MessageEmbed{Description: "Page one"}, 
		  &discordgo.MessageEmbed{Description: "Page two"},
		  &discordgo.MessageEmbed{Description: "Page three"})

	// Sets the footers of all added pages to their page numbers.
	p.SetPageFooters()

	// When the paginator is done listening set the colour to yellow
	p.ColourWhenDone = 0xffff

	// Stop listening for reaction events after five minutes
	p.Widget.Timeout = time.Minute * 5

	// Add a custom handler for the gun reaction.
	p.Widget.Handle("🔫", func(w *dgwidgets.Widget, r *discordgo.MessageReaction) {
		s.ChannelMessageSend(m.ChannelID, "Bang!")
	})

	p.Spawn()
}
```