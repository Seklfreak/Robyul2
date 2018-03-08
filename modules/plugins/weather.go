package plugins

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
	"github.com/shawntoffel/darksky"
)

type Weather struct {
	darkSkyClient darksky.DarkSky
}

const (
	googleMapsGeocodingEndpoint = "https://maps.googleapis.com/maps/api/geocode/json?language=en&key=%s&address=%s"
	darkSkyFriendlyForecast     = "https://darksky.net/forecast/%s,%s/si24"
	darkSkyHexColor             = "#2B86F3"
)

func (w *Weather) Commands() []string {
	return []string{
		"weather",
	}
}

func (w *Weather) Init(session *discordgo.Session) {
	w.darkSkyClient = darksky.New(helpers.GetConfig().Path("darksky.api_key").Data().(string))
}

func (w *Weather) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermWeather) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	var latResult float64
	var lngResult float64
	var addressResult string

	if content == "" {
		latResult, lngResult, addressResult = w.getLastLocation(msg.Author.ID)
		if latResult == 0 && lngResult == 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}

	if content != "" {
		geocodingUrl := fmt.Sprintf(googleMapsGeocodingEndpoint,
			helpers.GetConfig().Path("google.api_key").Data().(string),
			url.QueryEscape(content),
		)
		geocodingResult := helpers.GetJSON(geocodingUrl)
		locationChildren, err := geocodingResult.Path("results").Children()
		helpers.Relax(err)
		if geocodingResult.Path("status").Data().(string) != "OK" || len(locationChildren) <= 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.weather.address-not-found"))
			return
		}

		for _, location := range locationChildren {
			latResult = location.Path("geometry.location.lat").Data().(float64)
			lngResult = location.Path("geometry.location.lng").Data().(float64)
			addressResult = location.Path("formatted_address").Data().(string)
		}

		if addressResult == "" {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.weather.address-not-found"))
			return
		}
	}

	darkSkyForecast, err := w.darkSkyClient.Forecast(darksky.ForecastRequest{
		Latitude:  darksky.Measurement(latResult),
		Longitude: darksky.Measurement(lngResult),
		Options: darksky.ForecastRequestOptions{
			Exclude: "minutely,hourly,alerts,flags",
			Extend:  "",
			Lang:    "en",
			Units:   "si",
		},
	})
	metrics.DarkSkyRequests.Add(1)
	helpers.Relax(err)

	if darkSkyForecast.Currently.Summary == "" {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.weather.no-weather"))
		return
	}

	go func() {
		w.setLastLocation(msg.Author.ID, latResult, lngResult, addressResult)
	}()

	weatherEmbed := &discordgo.MessageEmbed{
		Title: helpers.GetTextF("plugins.weather.weather-embed-title", addressResult),
		URL: fmt.Sprintf(darkSkyFriendlyForecast,
			strconv.FormatFloat(latResult, 'f', -1, 64),
			strconv.FormatFloat(lngResult, 'f', -1, 64)),
		//Thumbnail: &discordgo.MessageEmbedThumbnail{URL: fmt.Sprintf(weatherIconsBaseUrl, darkSkyForecast.Currently.Icon)},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.weather.embed-footer"),
			IconURL: helpers.GetText("plugins.weather.embed-footer-imageurl"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Currently",
				Value: helpers.GetTextF("plugins.weather.current-weather-description",
					w.getWeatherEmoji(darkSkyForecast.Currently.Icon),
					darkSkyForecast.Currently.Summary,
					strconv.FormatFloat(float64(darkSkyForecast.Currently.Temperature), 'f', 1, 64),
					strconv.FormatFloat(float64(darkSkyForecast.Currently.Temperature)*1.8+32, 'f', 1, 64),
					strconv.FormatFloat(float64(darkSkyForecast.Currently.ApparentTemperature), 'f', 1, 64),
					strconv.FormatFloat(float64(darkSkyForecast.Currently.ApparentTemperature)*1.8+32, 'f', 1, 64),
					strconv.FormatFloat(float64(darkSkyForecast.Currently.WindSpeed), 'f', 1, 64),
					strconv.FormatFloat(float64(darkSkyForecast.Currently.WindSpeed)*2.23694, 'f', 1, 64),
					strconv.FormatFloat(float64(darkSkyForecast.Currently.Humidity)*100, 'f', 0, 64),
				),
				Inline: false,
			},
			{
				Name:   helpers.GetText("plugins.weather.week-title"),
				Value:  w.getWeatherEmoji(darkSkyForecast.Daily.Icon) + " " + darkSkyForecast.Daily.Summary,
				Inline: false,
			},
		},
		Color: helpers.GetDiscordColorFromHex(darkSkyHexColor),
	}

	_, err = helpers.SendEmbed(msg.ChannelID, weatherEmbed)
	helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
}

func (w *Weather) getWeatherEmoji(iconName string) (emoji string) {
	switch iconName {
	case "clear-day":
		return "☀"
	case "clear-night":
		return ""
	case "rain":
		return "🌧"
	case "snow":
		return "☃"
	case "sleet":
		return "🌃"
	case "wind":
		return "🌬"
	case "fog":
		return "🌁"
	case "cloudy":
		return "☁"
	case "partly-cloudy-day":
		return "⛅"
	case "partly-cloudy-night":
		return "☁"
	case "hail":
		return "🌨"
	case "thunderstorm":
		return "⛈"
	case "tornado":
		return "🌪"
	}
	return ""
}

func (w *Weather) setLastLocation(userID string, lat float64, lng float64, text string) (err error) {
	entry, err := w.getLastLocationEntry(userID)
	if err != nil || entry.ID == "" {
		_, err = rethink.Table(models.WeatherLastLocationsTable).Insert(models.WeatherLastLocation{
			UserID: userID,
			Lat:    lat,
			Lng:    lng,
			Text:   text,
		}).RunWrite(helpers.GetDB())
		return err
	}
	entry.Lat = lat
	entry.Lng = lng
	entry.Text = text
	_, err = rethink.Table(models.WeatherLastLocationsTable).Get(entry.ID).Update(entry).RunWrite(helpers.GetDB())
	return err
}

func (w *Weather) getLastLocation(userID string) (lat float64, lng float64, text string) {
	entry, err := w.getLastLocationEntry(userID)
	if err != nil {
		return 0, 0, ""
	}
	return entry.Lat, entry.Lng, entry.Text
}

func (w *Weather) getLastLocationEntry(userID string) (entry models.WeatherLastLocation, err error) {
	listCursor, err := rethink.Table(models.WeatherLastLocationsTable).Filter(
		rethink.Row.Field("user_id").Eq(userID),
	).Run(helpers.GetDB())
	if err != nil {
		return entry, err
	}
	defer listCursor.Close()
	err = listCursor.One(&entry)

	if err == rethink.ErrEmptyResult {
		return entry, errors.New("no weather last location entry")
	}
	return entry, err
}
