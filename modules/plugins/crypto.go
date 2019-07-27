package plugins

import (
	"sort"
	"strings"

	"time"

	"context"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/lucazulian/cryptocomparego"
	"github.com/sirupsen/logrus"
)

type cryptoAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next cryptoAction)

type Crypto struct {
	cryptoClient *cryptocomparego.Client
}

func (m *Crypto) Commands() []string {
	return []string{
		"crypto",
		"cryptocurrency",
	}
}

func (m *Crypto) Init(session *discordgo.Session) {
	// init crypto client
	m.cryptoClient = cryptocomparego.NewClient(helpers.DefaultClient)
}

func (m *Crypto) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermCrypto) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := m.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (m *Crypto) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) cryptoAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	return m.actionExchange
}

// [p]crypto [<from, eg BTC,BCH>] [<to, eg USD,EUR>]
func (m *Crypto) actionExchange(args []string, in *discordgo.Message, out **discordgo.MessageSend) cryptoAction {
	// default symbols
	fromSymbols := []string{"BTC", "BCH", "ETH", "LTC"}
	toSymbols := []string{"USD", "EUR", "KRW"}

	// parse custom symbols
	if len(args) >= 1 {
		fromSymbolsRaw := strings.Split(args[0], ",")
		if fromSymbolsRaw != nil && len(fromSymbolsRaw) > 0 {
			fromSymbols = make([]string, 0)
			for _, fromSymbolRaw := range fromSymbolsRaw {
				fromSymbols = append(fromSymbols, strings.ToUpper(strings.TrimSpace(fromSymbolRaw)))
			}
		}
	}
	if len(args) >= 2 {
		toSymbolsRaw := strings.Split(args[1], ",")
		if toSymbolsRaw != nil && len(toSymbolsRaw) > 0 {
			toSymbols = make([]string, 0)
			for _, toSymbolRaw := range toSymbolsRaw {
				toSymbols = append(toSymbols, strings.ToUpper(strings.TrimSpace(toSymbolRaw)))
			}
		}
	}

	// make exchange api request
	exchangeResults, _, err := m.cryptoClient.PriceMulti.List(context.Background(), &cryptocomparego.PriceMultiRequest{
		Fsyms:         fromSymbols,
		Tsyms:         toSymbols,
		ExtraParams:   helpers.DEFAULT_UA,
		TryConversion: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "There is no data for any of the") {
			*out = m.newMsg(helpers.GetText("bot.arguments.invalid"))
			return m.actionFinish
		}
	}
	helpers.Relax(err)

	// setup embed
	exchangeEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetText("plugins.crypto.embed-exchange-title"),
		Timestamp: time.Now().Format(time.RFC3339),
		Color:     helpers.GetDiscordColorFromHex("2b5a98"),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.crypto.embed-footer"),
			IconURL: helpers.GetText("plugins.crypto.embed-footer-imageurl"),
		},
		Fields: []*discordgo.MessageEmbedField{},
	}

	// custom sorting
	// USD before EUR before KRW before everything else
	// BTC before ETH before LTC before BCH before everything else
	sort.Slice(exchangeResults, func(i, j int) bool {
		if exchangeResults[i].Name == "USD" {
			return true
		}
		if exchangeResults[i].Name == "EUR" && exchangeResults[j].Name != "USD" {
			return true
		}
		if exchangeResults[i].Name == "KRW" && (exchangeResults[j].Name != "USD" && exchangeResults[j].Name != "KRW") {
			return true
		}
		if exchangeResults[i].Name == "BTC" {
			return true
		}
		if exchangeResults[i].Name == "ETH" && exchangeResults[j].Name != "BTC" {
			return true
		}
		if exchangeResults[i].Name == "LTC" && (exchangeResults[j].Name != "BTC" && exchangeResults[j].Name != "ETH") {
			return true
		}
		if exchangeResults[i].Name == "BCH" && (exchangeResults[j].Name != "BTC" && exchangeResults[j].Name != "ETH" && exchangeResults[j].Name != "LTC") {
			return true
		}
		return exchangeResults[i].Name < exchangeResults[j].Name
	})

	// add result
	for _, exchangeResult := range exchangeResults {
		var priceText string
		// custom sorting
		// USD before EUR before KRW before everything else
		// BTC before ETH before LTC before BCH before everything else
		sort.Slice(exchangeResult.Value, func(i, j int) bool {
			if exchangeResult.Value[i].Name == "USD" {
				return true
			}
			if exchangeResult.Value[i].Name == "EUR" && exchangeResult.Value[j].Name != "USD" {
				return true
			}
			if exchangeResult.Value[i].Name == "KRW" && (exchangeResult.Value[j].Name != "USD" && exchangeResult.Value[j].Name != "EUR") {
				return true
			}
			if exchangeResult.Value[i].Name == "BTC" {
				return true
			}
			if exchangeResult.Value[i].Name == "ETH" && exchangeResult.Value[j].Name != "BTC" {
				return true
			}
			if exchangeResult.Value[i].Name == "LTC" && (exchangeResult.Value[j].Name != "BTC" && exchangeResult.Value[j].Name != "ETH") {
				return true
			}
			if exchangeResult.Value[i].Name == "BCH" && (exchangeResult.Value[j].Name != "BTC" && exchangeResult.Value[j].Name != "ETH" && exchangeResult.Value[j].Name != "LTC") {
				return true
			}
			return exchangeResult.Value[i].Name < exchangeResult.Value[j].Name
		})
		// format prices
		for _, price := range exchangeResult.Value {
			priceText += price.Name + ": `" + humanize.FormatFloat("#,###.####", price.Value) + "`, "
		}
		priceText = strings.TrimRight(priceText, ", ")

		// add cryptocurrency
		newField := &discordgo.MessageEmbedField{
			Name:   "1 " + exchangeResult.Name,
			Value:  priceText,
			Inline: false,
		}
		exchangeEmbed.Fields = append(exchangeEmbed.Fields, newField)
	}

	// send result
	*out = &discordgo.MessageSend{
		Embed: exchangeEmbed,
	}
	return m.actionFinish
}

func (m *Crypto) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) cryptoAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (m *Crypto) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (m *Crypto) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (m *Crypto) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "crypto")
}
