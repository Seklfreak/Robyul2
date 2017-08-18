package plugins

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
)

type Weather struct{}

const (
	googleMapsGeocodingEndpoint string = "https://maps.googleapis.com/maps/api/geocode/json?language=en&key=%s&address=%s"
	darkSkyForecastRequest      string = "https://api.darksky.net/forecast/%s/%s,%s?exclude=minutely,hourly&lang=en&units=si"
	darkSkyFriendlyForecast     string = "https://darksky.net/forecast/%s,%s/si24"
	darkSkyHexColor             string = "#2B86F3"
)

type DarkSkyForecast struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	Offset    float64 `json:"offset"`
	Currently struct {
		Time                int     `json:"time"`
		Summary             string  `json:"summary"`
		Icon                string  `json:"icon"`
		PrecipIntensity     float64 `json:"precipIntensity"`
		PrecipProbability   float64 `json:"precipProbability"`
		PrecipType          string  `json:"precipType"`
		Temperature         float64 `json:"temperature"`
		ApparentTemperature float64 `json:"apparentTemperature"`
		DewPoint            float64 `json:"dewPoint"`
		Humidity            float64 `json:"humidity"`
		WindSpeed           float64 `json:"windSpeed"`
		WindBearing         int     `json:"windBearing"`
		Visibility          float64 `json:"visibility"`
		CloudCover          float64 `json:"cloudCover"`
		Pressure            float64 `json:"pressure"`
		Ozone               float64 `json:"ozone"`
	} `json:"currently"`
	Daily struct {
		Summary string `json:"summary"`
		Icon    string `json:"icon"`
		Data    []struct {
			Time                       int     `json:"time"`
			Summary                    string  `json:"summary"`
			Icon                       string  `json:"icon"`
			SunriseTime                int     `json:"sunriseTime"`
			SunsetTime                 int     `json:"sunsetTime"`
			MoonPhase                  float64 `json:"moonPhase"`
			PrecipIntensity            float64 `json:"precipIntensity"`
			PrecipIntensityMax         float64 `json:"precipIntensityMax"`
			PrecipIntensityMaxTime     int     `json:"precipIntensityMaxTime,omitempty"`
			PrecipProbability          float64 `json:"precipProbability"`
			PrecipType                 string  `json:"precipType,omitempty"`
			TemperatureMin             float64 `json:"temperatureMin"`
			TemperatureMinTime         int     `json:"temperatureMinTime"`
			TemperatureMax             float64 `json:"temperatureMax"`
			TemperatureMaxTime         int     `json:"temperatureMaxTime"`
			ApparentTemperatureMin     float64 `json:"apparentTemperatureMin"`
			ApparentTemperatureMinTime int     `json:"apparentTemperatureMinTime"`
			ApparentTemperatureMax     float64 `json:"apparentTemperatureMax"`
			ApparentTemperatureMaxTime int     `json:"apparentTemperatureMaxTime"`
			DewPoint                   float64 `json:"dewPoint"`
			Humidity                   float64 `json:"humidity"`
			WindSpeed                  float64 `json:"windSpeed"`
			WindBearing                int     `json:"windBearing"`
			Visibility                 float64 `json:"visibility,omitempty"`
			CloudCover                 float64 `json:"cloudCover"`
			Pressure                   float64 `json:"pressure"`
			Ozone                      float64 `json:"ozone"`
		} `json:"data"`
	} `json:"daily"`
	Flags struct {
		Sources       []string `json:"sources"`
		IsdStations   []string `json:"isd-stations"`
		MadisStations []string `json:"madis-stations"`
		Units         string   `json:"units"`
	} `json:"flags"`
}

func (w *Weather) Commands() []string {
	return []string{
		"weather",
	}
}

func (w *Weather) Init(session *discordgo.Session) {

}

func (w *Weather) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	if content == "" {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	geocodingUrl := fmt.Sprintf(googleMapsGeocodingEndpoint,
		helpers.GetConfig().Path("google.api_key").Data().(string),
		url.QueryEscape(content),
	)
	geocodingResult := helpers.GetJSON(geocodingUrl)
	locationChildren, err := geocodingResult.Path("results").Children()
	helpers.Relax(err)
	if geocodingResult.Path("status").Data().(string) != "OK" || len(locationChildren) <= 0 {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.weather.address-not-found"))
		return
	}

	var addressResult string
	var latResult float64
	var lngResult float64
	for _, location := range locationChildren {
		latResult = location.Path("geometry.location.lat").Data().(float64)
		lngResult = location.Path("geometry.location.lng").Data().(float64)
		addressResult = location.Path("formatted_address").Data().(string)
	}

	if addressResult == "" {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.weather.address-not-found"))
		return
	}

	darkSkyUrl := fmt.Sprintf(darkSkyForecastRequest,
		helpers.GetConfig().Path("darksky.api_key").Data().(string),
		strconv.FormatFloat(latResult, 'f', -1, 64),
		strconv.FormatFloat(lngResult, 'f', -1, 64))
	forecastResult := helpers.NetGet(darkSkyUrl)
	metrics.DarkSkyRequests.Add(1)
	var darkSkyForecast DarkSkyForecast
	err = json.Unmarshal(forecastResult, &darkSkyForecast)
	helpers.Relax(err)

	if darkSkyForecast.Currently.Summary == "" {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.weather.no-weather"))
		return
	}

	weatherEmbed := &discordgo.MessageEmbed{
		Title: helpers.GetTextF("plugins.weather.weather-embed-title", addressResult),
		URL: fmt.Sprintf(darkSkyFriendlyForecast,
			strconv.FormatFloat(latResult, 'f', -1, 64),
			strconv.FormatFloat(lngResult, 'f', -1, 64)),
		//Thumbnail: &discordgo.MessageEmbedThumbnail{URL: fmt.Sprintf(weatherIconsBaseUrl, darkSkyForecast.Currently.Icon)},
		Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.weather.embed-footer")},
		Description: helpers.GetTextF("plugins.weather.current-weather-description",
			darkSkyForecast.Currently.Summary,
			strconv.FormatFloat(darkSkyForecast.Currently.Temperature, 'f', 1, 64),
			strconv.FormatFloat(darkSkyForecast.Currently.Temperature*1.8+32, 'f', 1, 64),
			strconv.FormatFloat(darkSkyForecast.Currently.ApparentTemperature, 'f', 1, 64),
			strconv.FormatFloat(darkSkyForecast.Currently.ApparentTemperature*1.8+32, 'f', 1, 64),
			strconv.FormatFloat(darkSkyForecast.Currently.WindSpeed, 'f', 1, 64),
			strconv.FormatFloat(darkSkyForecast.Currently.WindSpeed*2.23694, 'f', 1, 64),
			strconv.FormatFloat(darkSkyForecast.Currently.Humidity*100, 'f', 0, 64),
		),
		Fields: []*discordgo.MessageEmbedField{
			{Name: helpers.GetText("plugins.weather.week-title"), Value: darkSkyForecast.Daily.Summary, Inline: false}},
		Color: helpers.GetDiscordColorFromHex(darkSkyHexColor),
	}

	_, err = session.ChannelMessageSendEmbed(msg.ChannelID, weatherEmbed)
	helpers.Relax(err)
}
