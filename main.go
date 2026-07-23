package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gtuk/discordwebhook"
	"github.com/rs/zerolog"
)

var webhookURL string

const mainLink = "https://subbacultcha.nl/events/"

type Event struct {
	Title     string
	Date      string
	Thumbnail string
	Location  string
	URL       string
}

type Monitor struct {
	cache  map[string]Event
	logger zerolog.Logger
}

func (m *Monitor) Refresh(link string, init bool) {
	resp, err := http.Get(link)

	if err != nil {
		m.logger.Err(err).Msg("getting events page")
		return
	} else if resp.StatusCode > 200 {
		m.logger.Error().Int("status code", resp.StatusCode)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		m.logger.Err(err).Msg("parsing events page")
		return
	}

	// iterate over cards
	doc.Find(".col-xs-12.col-sm-6.col-md-3.card.events-card").Each(func(i int, s *goquery.Selection) {
		title, err := s.Find(".info").Children().First().Children().First().Html()

		if err != nil {
			m.logger.Err(err).Msg("error getting title innerhtml")
		} else if _, exists := m.cache[title]; exists {
			// already in cache, skip
			return
		}

		// listing is not in cache, gather extra info and log

		date, err := s.Find(".date").First().Html()

		if err != nil {
			m.logger.Err(err).Msg("error getting date innerhtml")
			return
		}

		location, err := s.Find(".location").First().Html()

		if err != nil {
			m.logger.Err(err).Msg("error getting location innerhtml")
			return
		}

		thumbnail, exists := s.Find("img").First().Attr("src")

		if !exists {
			m.logger.Error().Msg("image doesn't have source")
			return
		}

		url, exists := s.Find("a").First().Attr("href")

		if !exists {
			m.logger.Err(err).Msg("error getting url")
			return
		}

		e := Event{
			title,
			date,
			thumbnail,
			location,
			url,
		}

		m.cache[title] = e

		if !init {
			m.logger.Info().Any("event", e).Msg("new event found!")
			m.Alert(e)
		}

	})

	// check for pagination

	loadMore := doc.Find(".next")

	if loadMore.Length() > 0 {
		nextPageLink, exists := loadMore.First().Attr("href")

		// sanity checks voor pagination links
		if exists && nextPageLink != "" && nextPageLink != link {
			m.Refresh(nextPageLink, init)
		}
	}
}

func (m *Monitor) Alert(e Event) {
	embed := discordwebhook.Embed{
		Title: &e.Title,
		Url:   &e.URL,
		Fields: &[]discordwebhook.Field{
			{
				Name:   ptr("Date"),
				Value:  &e.Date,
				Inline: ptr(true),
			},
			{
				Name:   ptr("Location"),
				Value:  &e.Location,
				Inline: ptr(true),
			},
		},
		Thumbnail: &discordwebhook.Thumbnail{
			Url: &e.Thumbnail,
		},
	}

	discordwebhook.SendMessage(webhookURL, discordwebhook.Message{Username: ptr("Subba monitor"), Embeds: &[]discordwebhook.Embed{embed}})
}

func NewMonitor() *Monitor {
	output := zerolog.ConsoleWriter{Out: os.Stdout}
	logger := zerolog.New(output).With().Timestamp().Logger()
	return &Monitor{
		cache:  map[string]Event{},
		logger: logger,
	}
}

func main() {
	webhookURL = os.Getenv("DISCORD_WEBHOOK")

	if webhookURL == "" {
		log.Fatal("No DISCORD_WEBHOOK environment variable found")
	}

	mon := NewMonitor()

	// init
	mon.Refresh(mainLink, true)

	mon.logger.Info().Int("Current event count", len(mon.cache)).Msg("Initialization complete")

	for {
		// refresh every 5 mins
		time.Sleep(time.Minute * 5)
		mon.Refresh(mainLink, false)
	}
}

func ptr[T any](v T) *T {
	return &v
}
