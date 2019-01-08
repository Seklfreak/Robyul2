package plugins

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"os"

	"os/exec"

	"io/ioutil"

	"strconv"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	cairo "github.com/ungerik/go-cairo"
)

type Spoiler struct {
	cachePath    string
	ffmpegBinary string
	env          []string
}

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

	s.cachePath = helpers.GetConfig().Path("cache_folder").Data().(string)

	s.ffmpegBinary, err = exec.LookPath("ffmpeg")
	if err != nil {
		cache.GetLogger().WithField("module", "spoiler").Infoln("module disabled: ffmpeg not found")
	}

	s.env = os.Environ()

	BuiltinEmotePattern, err = regexp.Compile(`[\x{1F600}-\x{1F6FF}|[\x{2600}-\x{26FF}]`)
	helpers.Relax(err)
	CustomEmotePattern, err = regexp.Compile(`\<a?(\:[^:]+\:)[0-9]+\>`)
	helpers.Relax(err)

}

func (s *Spoiler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermSpoiler) {
		return
	}

	if s.ffmpegBinary == "" {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	content = strings.TrimSpace(content)

	if len(content) <= 0 {
		_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		helpers.Relax(err)
		return
	}

	session.ChannelMessageDelete(msg.ChannelID, msg.ID)

	blankFrameFilename := s.cachePath + "0-" + msg.ID + ".png"
	contentFrameFilename := s.cachePath + "1-" + msg.ID + ".png"
	videoFrameFilename := s.cachePath + "output-" + msg.ID + ".webm"

	allUnicodeEmotes := BuiltinEmotePattern.FindAllString(content, -1)
	for _, unicodeEmote := range allUnicodeEmotes {
		content = strings.Replace(content, unicodeEmote, "[emote]", 1)
	}

	allCustomEmotes := CustomEmotePattern.FindAllStringSubmatch(content, -1)
	for _, customEmoteSubstrings := range allCustomEmotes {
		content = strings.Replace(content, customEmoteSubstrings[0], customEmoteSubstrings[1], 1)
	}

	fontSize := 15.0
	lines := s.breakIntoLines(content, fontSize)
	lineHeight := 40.0
	contentSize := (float64(len(lines)) + float64(0.5)) * float64(lineHeight/2)
	top := 45.0
	bottom := 32.0
	height := top + contentSize + bottom

	surface := cairo.NewSurface(cairo.FORMAT_ARGB32, int(SpoilerWidth), int(height))
	surface.SetSourceRGB(0, 0, 0)
	surface.Rectangle(0, 0, SpoilerWidth, height)
	surface.Fill()
	//surface.SelectFontFace("serif", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	surface.SelectFontFace("assets/SourceSansPro-Regular.ttf", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	surface.SetFontSize(fontSize)
	surface.SetSourceRGB(1.0, 1.0, 1.0)
	for i, line := range lines {
		surface.MoveTo(10.0, top+(lineHeight/2)*float64(i+1))
		surface.ShowText(line)
	}
	status := surface.WriteToPNG(contentFrameFilename)
	if status != cairo.STATUS_SUCCESS {
		helpers.RelaxLog(errors.New("failed to write surface"))
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.spoiler.error-generic"))
		return
	}

	surface = cairo.NewSurface(cairo.FORMAT_ARGB32, int(SpoilerWidth), int(height))
	surface.SetSourceRGB(0, 0, 0)
	surface.Rectangle(0, 0, SpoilerWidth, height)
	surface.Fill()
	surface.SelectFontFace("assets/SourceSansPro-Regular.ttf", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	surface.SetFontSize(fontSize)
	surface.SetSourceRGB(1.0, 1.0, 1.0)
	surface.MoveTo(99.0, ((height/2)+23.0)+(lineHeight/2)*float64(0+1))
	surface.ShowText("Press play to reveal the spoiler")
	status = surface.WriteToPNG(blankFrameFilename)
	if status != cairo.STATUS_SUCCESS {
		helpers.RelaxLog(errors.New("failed to write surface"))
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.spoiler.error-generic"))
		return
	}

	cmdArgs := []string{
		"-f",
		"image2",
		"-i",
		s.cachePath + "%01d-" + msg.ID + ".png",
		"-s",
		strconv.Itoa(int(SpoilerWidth)) + "x" + strconv.Itoa(int(height)),
		"-c:v",
		"libvpx-vp9",
		"-pix_fmt",
		"yuva420p",
		"-framerate",
		"10",
		videoFrameFilename,
	}
	imgCmd := exec.Command(s.ffmpegBinary, cmdArgs...)
	imgCmd.Env = s.env
	var out bytes.Buffer
	var stderr bytes.Buffer
	imgCmd.Stdout = &out
	imgCmd.Stderr = &stderr
	err := imgCmd.Run()
	if err != nil {
		helpers.RelaxLog(errors.New(fmt.Sprint(err) + ": " + stderr.String()))
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.spoiler.error-generic"))
		return
	}

	os.Remove(blankFrameFilename)
	os.Remove(contentFrameFilename)

	videoData, err := ioutil.ReadFile(videoFrameFilename)
	helpers.Relax(err)

	_, err = helpers.SendComplex(
		msg.ChannelID, &discordgo.MessageSend{
			Content: fmt.Sprintf("<@%s> said:", msg.Author.ID),
			Files: []*discordgo.File{
				{
					Name:   "Robyul-Spoiler.webm",
					Reader: bytes.NewReader(videoData),
				},
			},
		})
	helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

	os.Remove(videoFrameFilename)
}

func (s *Spoiler) breakIntoLines(text string, fontSize float64) []string {
	surface := cairo.NewSurface(cairo.FORMAT_ARGB32, int(SpoilerWidth), 1)
	surface.SelectFontFace("assets/SourceSansPro-Regular.ttf", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	surface.SetFontSize(fontSize)

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
