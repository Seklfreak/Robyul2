package dgwidgets

import (
	"errors"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

// error vars
var (
	ErrAlreadyRunning   = errors.New("err: Widget already running")
	ErrIndexOutOfBounds = errors.New("err: Index is out of bounds")
	ErrNilMessage       = errors.New("err: Message is nil")
	ErrNilEmbed         = errors.New("err: embed is nil")
	ErrNotRunning       = errors.New("err: not running")
)

// WidgetHandler ...
type WidgetHandler func(*Widget, *discordgo.MessageReaction)

// Widget is a message embed with reactions for buttons.
// Accepts custom handlers for reactions.
type Widget struct {
	sync.Mutex
	Embed     *discordgo.MessageEmbed
	Message   *discordgo.Message
	ChannelID string
	Timeout   time.Duration
	Close     chan bool

	// Handlers binds emoji names to functions
	Handlers map[string]WidgetHandler
	// keys stores the handlers keys in the order they were added
	Keys []string

	// Delete reactions after they are added
	DeleteReactions bool
	// Only allow listed users to use reactions.
	UserWhitelist []string

	running bool
}

// NewWidget returns a pointer to a Widget object
//    ses      : discordgo session
//    channelID: channelID to spawn the widget on
func NewWidget(channelID string, userID string, embed *discordgo.MessageEmbed) *Widget {
	return &Widget{
		ChannelID:       channelID,
		Keys:            []string{},
		Handlers:        map[string]WidgetHandler{},
		Close:           make(chan bool),
		DeleteReactions: true,
		Embed:           embed,
		Timeout:         time.Minute * 5,
		UserWhitelist:   []string{userID},
	}
}

// isUserAllowed returns true if the user is allowed
// to use this widget.
func (w *Widget) isUserAllowed(userID string) bool {
	if w.UserWhitelist == nil || len(w.UserWhitelist) == 0 {
		return true
	}
	for _, user := range w.UserWhitelist {
		if user == userID {
			return true
		}
	}
	return false
}

// Spawn spawns the widget in channel w.ChannelID
func (w *Widget) Spawn() error {
	if w.Running() {
		return ErrAlreadyRunning
	}
	w.running = true
	defer func() {
		w.running = false
	}()

	if w.Embed == nil {
		return ErrNilEmbed
	}

	startTime := time.Now()

	// Create initial message.
	msg, err := helpers.SendEmbed(w.ChannelID, w.Embed)
	if err != nil {
		return err
	}
	w.Message = msg[0]

	// Add reaction buttons
	for _, v := range w.Keys {
		cache.GetSession().MessageReactionAdd(w.Message.ChannelID, w.Message.ID, v)
	}

	var reaction *discordgo.MessageReaction
	for {
		// Navigation timeout enabled
		if w.Timeout != 0 {
			select {
			case k := <-nextMessageReactionAddC(cache.GetSession()):
				reaction = k.MessageReaction
			case <-time.After(startTime.Add(w.Timeout).Sub(time.Now())):
				return nil
			case <-w.Close:
				return nil
			}
		} else /*Navigation timeout not enabled*/ {
			select {
			case k := <-nextMessageReactionAddC(cache.GetSession()):
				reaction = k.MessageReaction
			case <-w.Close:
				return nil
			}
		}

		// Ignore reactions sent by bot
		if reaction.MessageID != w.Message.ID || cache.GetSession().State.User.ID == reaction.UserID {
			continue
		}

		if v, ok := w.Handlers[reaction.Emoji.Name]; ok {
			if w.isUserAllowed(reaction.UserID) {
				go v(w, reaction)
			}
		}

		if w.DeleteReactions {
			go func() {
				if w.isUserAllowed(reaction.UserID) {
					time.Sleep(time.Millisecond * 250)
					cache.GetSession().MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
				}
			}()
		}
	}
}

// Handle adds a handler for the given emoji name
//    emojiName: The unicode value of the emoji
//    handler  : handler function to call when the emoji is clicked
//               func(*Widget, *discordgo.MessageReaction)
func (w *Widget) Handle(emojiName string, handler WidgetHandler) error {
	if _, ok := w.Handlers[emojiName]; !ok {
		w.Keys = append(w.Keys, emojiName)
		w.Handlers[emojiName] = handler
	}
	// if the widget is running, append the added emoji to the message.
	if w.Running() && w.Message != nil {
		return cache.GetSession().MessageReactionAdd(w.Message.ChannelID, w.Message.ID, emojiName)
	}
	return nil
}

// QueryInput querys the user with ID `id` for input
//    prompt : Question prompt
//    userID : UserID to get message from
//    timeout: How long to wait for the user's response
func (w *Widget) QueryInput(prompt string, userID string, timeout time.Duration) (*discordgo.Message, error) {
	msg, err := helpers.SendMessage(w.ChannelID, "<@"+userID+"> "+prompt)
	if err != nil {
		return nil, err
	}
	defer func() {
		cache.GetSession().ChannelMessageDelete(msg[0].ChannelID, msg[0].ID)
	}()

	timeoutChan := make(chan int)
	go func() {
		time.Sleep(timeout)
		timeoutChan <- 0
	}()

	for {
		select {
		case usermsg := <-nextMessageCreateC(cache.GetSession()):
			if usermsg.Author.ID != userID {
				continue
			}
			cache.GetSession().ChannelMessageDelete(usermsg.ChannelID, usermsg.ID)
			return usermsg.Message, nil
		case <-timeoutChan:
			return nil, errors.New("Timed out")
		}
	}
}

// Running returns w.running
func (w *Widget) Running() bool {
	w.Lock()
	running := w.running
	w.Unlock()
	return running
}

// UpdateEmbed updates the embed object and edits the original message
//    embed: New embed object to replace w.Embed
func (w *Widget) UpdateEmbed(embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	if w.Message == nil {
		return nil, ErrNilMessage
	}
	return helpers.EditEmbed(w.ChannelID, w.Message.ID, embed)
}
