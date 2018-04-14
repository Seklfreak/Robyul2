package plugins

import (
	"encoding/base64"
	"strings"

	"fmt"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/kairgo"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type facialRecognitionAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next facialRecognitionAction)

type FacialRecognition struct {
	kairosClient *kairgo.Kairos
	galleryName  string
}

func (m *FacialRecognition) Commands() []string {
	return []string{
		"facialrecognition",
		"whodis",
	}
}

func (m *FacialRecognition) Init(session *discordgo.Session) {
	var err error
	appId := helpers.GetConfig().Path("kairos.app_id").Data().(string)
	key := helpers.GetConfig().Path("kairos.key").Data().(string)
	m.galleryName = helpers.GetConfig().Path("kairos.gallery").Data().(string)

	if appId != "" && key != "" {
		m.kairosClient, err = kairgo.New("", appId, key, nil)
		helpers.Relax(err)
	}
}

func (m *FacialRecognition) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermStats) {
		return
	}

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := m.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (m *FacialRecognition) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) facialRecognitionAction {
	if m.kairosClient == nil {
		return nil
	}

	if len(args) >= 1 {
		switch args[0] {
		case "train":
			return m.actionTrain
		case "status":
			return m.actionStatus
		case "reset":
			return m.actionReset
		}
	}

	return m.actionRecognise
}

// [p]facialrecognition
func (m *FacialRecognition) actionRecognise(args []string, in *discordgo.Message, out **discordgo.MessageSend) facialRecognitionAction {
	if in.Attachments == nil || len(in.Attachments) <= 0 {
		return nil
	}

	cache.GetSession().ChannelTyping(in.ChannelID)

	response, err := m.kairosClient.Recognize(in.Attachments[0].URL, m.galleryName, "", "0.60", 3)
	helpers.Relax(err)

	if response.Errors != nil && len(response.Errors) > 0 {
		for _, kairosError := range response.Errors {
			if kairosError.ErrCode == 5002 { // no faces found in the image
				*out = m.newMsg("plugins.facialrecognition.recognise-not-found")
				return m.actionFinish
			}
			helpers.Relax(errors.New(fmt.Sprintf("%d: %s", kairosError.ErrCode, kairosError.Message)))
		}
	}

	var message string
	for _, images := range response.Images {
		message += fmt.Sprintf("x: %d, y: %d: ", images.Transaction.TopLeftX, images.Transaction.TopLeftY)
		postedIds := make([]string, 0)
	NextCandidate:
		for _, candidate := range images.Candidates {

			subject := m.readKairosKey(candidate.SubjectID)
			if subject.Name == "" || subject.GroupName == "" {
				continue NextCandidate
			}

			for _, postedId := range postedIds {
				if postedId == subject.Name+subject.GroupName+subject.Gender {
					continue NextCandidate
				}
			}

			message += fmt.Sprintf("%2.0f%%: %s's %s, ", candidate.Confidence*100, subject.Name, subject.GroupName)
			postedIds = append(postedIds, subject.Name+subject.GroupName+subject.Gender)
		}
		message = strings.TrimRight(message, ", ")
		message += "\n"
	}

	if message == "" {
		message = "plugins.facialrecognition.recognise-not-found"
	}

	*out = m.newMsg(message)
	return m.actionFinish
}

// [p]facialrecognition train
func (m *FacialRecognition) actionTrain(args []string, in *discordgo.Message, out **discordgo.MessageSend) facialRecognitionAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	// TODO: store same Idols together somehow? But we want to keep in sync with updates…
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = m.newMsg("bot.robyulmod.no_permission")
		return m.actionFinish
	}

	quitChannel := helpers.StartTypingLoop(in.ChannelID)
	defer func() { quitChannel <- 0 }()

	enrolledResponse, _ := m.kairosClient.ViewGallery(m.galleryName)

	var biasEntries []models.BiasGameIdolEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.BiasGameIdolsTable).Find(nil)).All(&biasEntries)
	helpers.Relax(err)

	var enrolled int

NextBiasEntry:
	for _, biasEntry := range biasEntries {
		if enrolledResponse != nil && enrolledResponse.SubjectIDs != nil && len(enrolledResponse.SubjectIDs) > 0 {
			for _, enrolledID := range enrolledResponse.SubjectIDs {
				if enrolledID == m.getKairosKey(biasEntry) {
					continue NextBiasEntry
				}
			}
		}

		imageData, err := helpers.RetrieveFileWithoutLogging(biasEntry.ObjectName)
		helpers.Relax(err)
		mimeType, _ := helpers.SniffMime(imageData)
		if mimeType != "image/jpeg" && mimeType != "image/png" {
			continue NextBiasEntry
		}

		base64Text := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(imageData)

	RetryEnroll:
		response, err := m.kairosClient.Enroll(base64Text, m.getKairosKey(biasEntry), m.galleryName, "", false)
		if err != nil {
			if strings.Contains(err.Error(), "invalid character") { // api limit?
				m.logger().Infof("hit rate limit enrolling %s's %s as %s, %s, retrying in 10 seconds",
					biasEntry.Name, biasEntry.GroupName, m.getKairosKey(biasEntry), err.Error())
				time.Sleep(time.Second * 10)
				goto RetryEnroll
			}
			helpers.Relax(err)
		}

		if response.Errors != nil && len(response.Errors) > 0 {
			for _, kairosError := range response.Errors {
				if kairosError.ErrCode == 5002 { // no faces found in the image
					m.logger().Infof("skipped enrolling %s's %s as %s for enrolling, no face found",
						biasEntry.Name, biasEntry.GroupName, m.getKairosKey(biasEntry))
					continue NextBiasEntry
				}
				if kairosError.ErrCode == 5010 { // too many faces in image
					m.logger().Infof("skipped enrolling %s's %s as %s for enrolling, too many faces in image",
						biasEntry.Name, biasEntry.GroupName, m.getKairosKey(biasEntry))
					continue NextBiasEntry
				}
				// TODO: cache skipped entries somewhere, to not try to enroll them every time
				helpers.Relax(errors.New(fmt.Sprintf("%d: %s", kairosError.ErrCode, kairosError.Message)))
			}
		}

		m.logger().Infof("enrolled %s's %s as %s",
			biasEntry.Name, biasEntry.GroupName, m.getKairosKey(biasEntry))
		enrolled++
	}

	quitChannel <- 0

	*out = m.newMsg(helpers.GetTextF("plugins.facialrecognition.train-success", enrolled))
	return m.actionFinish
}

// [p]facialrecognition status
func (m *FacialRecognition) actionStatus(args []string, in *discordgo.Message, out **discordgo.MessageSend) facialRecognitionAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = m.newMsg("bot.robyulmod.no_permission")
		return m.actionFinish
	}

	response, err := m.kairosClient.ViewGallery(m.galleryName)
	helpers.Relax(err)

	if response.Errors != nil && len(response.Errors) > 0 {
		for _, kairosError := range response.Errors {
			if kairosError.ErrCode == 5004 { // gallery name not found
				*out = m.newMsg("plugins.facialrecognition.status-none")
				return m.actionFinish
			}
			helpers.Relax(errors.New(fmt.Sprintf("%d: %s", kairosError.ErrCode, kairosError.Message)))
		}
	}

	*out = m.newMsg(helpers.GetTextF("plugins.facialrecognition.status", len(response.SubjectIDs)))
	return m.actionFinish
}

// [p]facialrecognition reset
func (m *FacialRecognition) actionReset(args []string, in *discordgo.Message, out **discordgo.MessageSend) facialRecognitionAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = m.newMsg("bot.robyulmod.no_permission")
		return m.actionFinish
	}

	response, err := m.kairosClient.RemoveGallery(m.galleryName)
	helpers.Relax(err)

	if response.Errors != nil && len(response.Errors) > 0 {
		for _, kairosError := range response.Errors {
			helpers.Relax(errors.New(fmt.Sprintf("%d: %s", kairosError.ErrCode, kairosError.Message)))
		}
	}

	*out = m.newMsg("plugins.facialrecognition.reset-success")
	return m.actionFinish
}

func (m *FacialRecognition) getKairosKey(entry models.BiasGameIdolEntry) (key string) {
	return helpers.MdbIdToHuman(entry.ID)
}

func (m *FacialRecognition) readKairosKey(key string) (entry models.BiasGameIdolEntry) {
	err := helpers.MdbOne(
		helpers.MdbCollection(models.BiasGameIdolsTable).Find(bson.M{"_id": helpers.HumanToMdbId(key)}),
		&entry,
	)
	if !helpers.IsMdbNotFound(err) {
		helpers.RelaxLog(err)
	}
	return entry
}

func (m *FacialRecognition) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) facialRecognitionAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (m *FacialRecognition) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (m *FacialRecognition) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (m *FacialRecognition) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "facialrecognition")
}
