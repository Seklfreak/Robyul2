package dgwidgets

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

// Paginator provides a method for creating a navigatable embed
type Paginator struct {
	sync.Mutex
	Pages []*discordgo.MessageEmbed
	Index int

	// Loop back to the beginning or end when on the first or last page.
	Loop   bool
	Widget *Widget

	FooterPrefix string

	DeleteMessageWhenDone   bool
	DeleteReactionsWhenDone bool
	Colour                  int
	ColourWhenDone          int

	lockToUser bool

	running bool
}

// NewPaginator returns a new Paginator
//    ses      : discordgo session
//    channelID: channelID to spawn the paginator on
func NewPaginator(guildID, channelID, userID string) *Paginator {
	p := &Paginator{
		Pages:                   []*discordgo.MessageEmbed{},
		Index:                   0,
		Loop:                    false,
		DeleteMessageWhenDone:   false,
		DeleteReactionsWhenDone: true,
		Colour:                  helpers.GetDiscordColorFromHex("73d016"), // lime green
		ColourWhenDone:          helpers.GetDiscordColorFromHex("ffb80a"), // orange
		Widget:                  NewWidget(guildID, channelID, userID, nil),
	}
	p.addHandlers()

	return p
}

func (p *Paginator) addHandlers() {
	p.Widget.Handle(NavBeginning, func(w *Widget, r *discordgo.MessageReaction) {
		if err := p.Goto(0); err == nil {
			p.Update()
		}
	})
	p.Widget.Handle(NavLeft, func(w *Widget, r *discordgo.MessageReaction) {
		if err := p.PreviousPage(); err == nil {
			p.Update()
		}
	})
	p.Widget.Handle(NavRight, func(w *Widget, r *discordgo.MessageReaction) {
		if err := p.NextPage(); err == nil {
			p.Update()
		}
	})
	p.Widget.Handle(NavEnd, func(w *Widget, r *discordgo.MessageReaction) {
		if err := p.Goto(len(p.Pages) - 1); err == nil {
			p.Update()
		}
	})
	p.Widget.Handle(NavNumbers, func(w *Widget, r *discordgo.MessageReaction) {
		if msg, err := w.QueryInput("which page would you like to open? <:blobidea:317047867036663809>", r.UserID, 30*time.Second); err == nil {
			if n, err := strconv.Atoi(msg.Content); err == nil {
				p.Goto(n - 1)
				p.Update()
			}
		}
	})
}

// Spawn spawns the paginator in channel p.ChannelID
func (p *Paginator) Spawn() error {
	if p.Running() {
		return ErrAlreadyRunning
	}
	p.Lock()
	p.running = true
	p.Unlock()

	defer func() {
		p.Lock()
		p.running = false
		p.Unlock()

		// Delete Message when done
		if p.DeleteMessageWhenDone && p.Widget.Message != nil {
			cache.GetSession().SessionForGuildS(p.Widget.GuildID).ChannelMessageDelete(p.Widget.Message.ChannelID, p.Widget.Message.ID)
		} else {
			p.Update()
		}

		// Delete reactions when done
		if p.DeleteReactionsWhenDone && p.Widget.Message != nil {
			cache.GetSession().SessionForGuildS(p.Widget.GuildID).MessageReactionsRemoveAll(p.Widget.ChannelID, p.Widget.Message.ID)
		}
	}()

	page, err := p.Page()
	if err != nil {
		return err
	}
	p.Widget.Embed = page

	return p.Widget.Spawn()
}

// Add a page to the paginator
//    embed: embed page to add.
func (p *Paginator) Add(embeds ...*discordgo.MessageEmbed) {
	p.Pages = append(p.Pages, embeds...)
}

// Page returns the page of the current index
func (p *Paginator) Page() (*discordgo.MessageEmbed, error) {
	p.Lock()
	defer p.Unlock()

	if p.Index < 0 || p.Index >= len(p.Pages) {
		return nil, ErrIndexOutOfBounds
	}

	if p.Pages[p.Index].Color == 0 {
		p.Pages[p.Index].Color = p.Colour
	}

	var newTitle, newFooterText, newFooterIconURL string
	if p.Widget != nil && len(p.Widget.UserWhitelist) >= 1 {
		author, err := helpers.GetUser(p.Widget.UserWhitelist[0])
		if err == nil {
			newTitle += "@" + author.Username
			if p.Pages[p.Index].Title != "" {
				newTitle += ": "
			}
		}
	}
	newTitle += p.Pages[p.Index].Title
	if p.Pages[p.Index].Footer != nil {
		if p.Pages[p.Index].Footer.Text != "" {
			newFooterText += p.Pages[p.Index].Footer.Text + ""
		}
		if p.Pages[p.Index].Footer.IconURL != "" {
			newFooterIconURL = p.Pages[p.Index].Footer.IconURL
		}
	}
	newFooterText += fmt.Sprintf("Page %d out of %d.", p.Index+1, len(p.Pages))
	if p.running {
		newFooterText += " Use the arrows below to navigate."
	}

	pageCopy := *p.Pages[p.Index]
	if newTitle != "" {
		pageCopy.Title = newTitle
	}
	if pageCopy.Footer == nil {
		pageCopy.Footer = &discordgo.MessageEmbedFooter{}
	}
	if newFooterText != "" {
		pageCopy.Footer.Text = newFooterText
	}
	if newFooterIconURL != "" {
		pageCopy.Footer.IconURL = newFooterIconURL
	}

	if !p.running && p.ColourWhenDone >= 0 {
		pageCopy.Color = p.ColourWhenDone
	}

	return &pageCopy, nil
}

// NextPage sets the page index to the next page
func (p *Paginator) NextPage() error {
	p.Lock()
	defer p.Unlock()

	if p.Index+1 >= 0 && p.Index+1 < len(p.Pages) {
		p.Index++
		return nil
	}

	// Set the queue back to the beginning if Loop is enabled.
	if p.Loop {
		p.Index = 0
		return nil
	}

	return ErrIndexOutOfBounds
}

// PreviousPage sets the current page index to the previous page.
func (p *Paginator) PreviousPage() error {
	p.Lock()
	defer p.Unlock()

	if p.Index-1 >= 0 && p.Index-1 < len(p.Pages) {
		p.Index--
		return nil
	}

	// Set the queue back to the beginning if Loop is enabled.
	if p.Loop {
		p.Index = len(p.Pages) - 1
		return nil
	}

	return ErrIndexOutOfBounds
}

// Goto jumps to the requested page index
//    index: The index of the page to go to
func (p *Paginator) Goto(index int) error {
	p.Lock()
	defer p.Unlock()
	if index < 0 || index >= len(p.Pages) {
		return ErrIndexOutOfBounds
	}
	p.Index = index
	return nil
}

// Update updates the message with the current state of the paginator
func (p *Paginator) Update() error {
	if p.Widget.Message == nil {
		return ErrNilMessage
	}

	page, err := p.Page()
	if err != nil {
		return err
	}

	_, err = p.Widget.UpdateEmbed(page)
	return err
}

// Running returns the running status of the paginator
func (p *Paginator) Running() bool {
	p.Lock()
	running := p.running
	p.Unlock()
	return running
}
