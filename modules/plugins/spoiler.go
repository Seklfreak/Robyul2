package plugins

import (
    "github.com/bwmarrin/discordgo"
    "github.com/ungerik/go-cairo"
    "strings"
    "image"
    "image/gif"
    "github.com/Seklfreak/Robyul2/helpers"
    "bytes"
    "image/png"
    "fmt"
    "regexp"
)

type Spoiler struct{}

var (
    BuiltinEmotePattern *regexp.Regexp
    CustomEmotePattern  *regexp.Regexp
)

const (
    SpoilerWidth = float64(400)
)

func (s *Spoiler) Commands() []string {
    return []string{
        "spoiler",
    }
}

func (s *Spoiler) Init(session *discordgo.Session) {
    var err error
    BuiltinEmotePattern, err = regexp.Compile(`[\x{1F600}-\x{1F6FF}|[\x{2600}-\x{26FF}]`)
    helpers.Relax(err)
    CustomEmotePattern, err = regexp.Compile(`\<(\:[^:]+\:)[0-9]+\>`)
    helpers.Relax(err)

}

func (s *Spoiler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelTyping(msg.ChannelID)

    content = strings.TrimSpace(content)

    if len(content) <= 0 {
        _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
        helpers.Relax(err)
        return
    }

    session.ChannelMessageDelete(msg.ChannelID, msg.ID)

    allUnicodeEmotes := BuiltinEmotePattern.FindAllString(content, -1)
    for _, unicodeEmote := range allUnicodeEmotes {
        content = strings.Replace(content, unicodeEmote, "[emote]", 1)
    }

    allCustomEmotes := CustomEmotePattern.FindAllStringSubmatch(content, -1)
    for _, customEmoteSubstrings := range allCustomEmotes {
        content = strings.Replace(content, customEmoteSubstrings[0], customEmoteSubstrings[1], 1)
    }

    lines := s.breakIntoLines(content)
    lineHeight := float64(40)
    height := (float64(len(lines)) + float64(0.5)) * float64(lineHeight/2)

    surface := cairo.NewSurface(cairo.FORMAT_ARGB32, int(SpoilerWidth), int(height))
    surface.SetSourceRGB(0.20, 0.20, 0.20)
    surface.Rectangle(0, 0, SpoilerWidth, height);
    surface.Fill();
    //surface.SelectFontFace("serif", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
    surface.SelectFontFace("assets/SourceSansPro-Regular.ttf", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
    surface.SetFontSize(13.0)
    surface.SetSourceRGB(1.0, 1.0, 1.0)
    for i, line := range lines {
        surface.MoveTo(10.0, (lineHeight/2)*float64(i+1))
        surface.ShowText(line)
    }
    pngBytes, _ := surface.WriteToPNGStream()
    decodedImage, err := png.Decode(bytes.NewReader(pngBytes))
    helpers.Relax(err)
    buf := bytes.Buffer{}
    err = gif.Encode(&buf, decodedImage, nil)
    helpers.Relax(err)
    tmpimg, err := gif.Decode(&buf)
    helpers.Relax(err)

    surface = cairo.NewSurface(cairo.FORMAT_ARGB32, int(SpoilerWidth), int(height))
    surface.SetSourceRGB(0.20, 0.20, 0.20)
    surface.Rectangle(0, 0, SpoilerWidth, height);
    surface.Fill();
    surface.SelectFontFace("assets/SourceSansPro-Regular.ttf", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
    surface.SetFontSize(13.0)
    surface.SetSourceRGB(1.0, 1.0, 1.0)
    surface.MoveTo(10.0, (lineHeight/2)*float64(0+1))
    surface.ShowText("( Hover to reveal spoiler )")
    pngBytes, _ = surface.WriteToPNGStream()
    decodedImage, err = png.Decode(bytes.NewReader(pngBytes))
    helpers.Relax(err)
    buf = bytes.Buffer{}
    err = gif.Encode(&buf, decodedImage, nil)
    helpers.Relax(err)
    blankFrame, err := gif.Decode(&buf)
    helpers.Relax(err)

    outGif := &gif.GIF{}
    outGif.Image = append(outGif.Image, blankFrame.(*image.Paletted))
    outGif.Delay = append(outGif.Delay, 0)
    outGif.Image = append(outGif.Image, tmpimg.(*image.Paletted))
    outGif.Delay = append(outGif.Delay, 100*60*60*24)
    outGif.LoopCount = 1
    // hacky workaround because gifs can not not loop in go: https://github.com/golang/go/issues/15768

    buf = bytes.Buffer{}
    err = gif.EncodeAll(&buf, outGif)
    helpers.Relax(err)

    _, err = session.ChannelFileSendWithMessage(
        msg.ChannelID,
        fmt.Sprintf("<@%s> said:", msg.Author.ID),
        "spoiler.gif", bytes.NewReader(buf.Bytes()))
    helpers.Relax(err)
}

func (s *Spoiler) breakIntoLines(text string) []string {
    surface := cairo.NewSurface(cairo.FORMAT_ARGB32, int(SpoilerWidth), 1)
    surface.SelectFontFace("assets/SourceSansPro-Regular.ttf", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
    surface.SetFontSize(13.0)

    linesSplit := strings.Split(text, "\n")
    resultLines := make([]string, 0)
    for _, line := range linesSplit {
        words := strings.Split(line, " ")
        newLine := ""
        for _, word := range words {
            extends := surface.TextExtents(newLine + word)
            if extends.Width >= float64(400-20) {
                resultLines = append(resultLines, newLine)
                newLine = word
            } else {
                newLine += word + " "
            }
        }
        resultLines = append(resultLines, newLine)
    }
    return resultLines
}
