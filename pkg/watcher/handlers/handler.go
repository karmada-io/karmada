package handlers

import (
	"github.com/karmada-io/karmada/pkg/watcher/config"
	"github.com/karmada-io/karmada/pkg/watcher/event"
)

// Handler is implemented by any handler.
// The Handle method is used to process event
type Handler interface {
	Init(c *config.Config) error
	Handle(e event.Event)
	Events() <-chan event.Event
}

// Map maps each event handler function to a name for easily lookup
var Map = map[string]Handler{
	"default": &Default{},
	// "slack":        &slack.Slack{},
	// "slackwebhook": &slackwebhook.SlackWebhook{},
	// "hipchat":      &hipchat.Hipchat{},
	// "mattermost":   &mattermost.Mattermost{},
	// "flock":        &flock.Flock{},
	// "webhook":      &webhook.Webhook{},
	// "ms-teams":     &msteam.MSTeams{},
	// "smtp":         &smtp.SMTP{},
	// "lark":         &lark.Webhook{},
}

// Default handler implements Handler interface,
// print each event with JSON format
type Default struct {
	events      chan event.Event
	eventBuffer int
}

// Init initializes handler configuration
// Do nothing for default handler
func (d *Default) Init(c *config.Config) error {
	if c.Handler.EventBuffer <= 0 {
		d.eventBuffer = 10
	} else {
		d.eventBuffer = c.Handler.EventBuffer
	}
	if d.events == nil {
		d.events = make(chan event.Event, d.eventBuffer)
	}
	return nil
}

// Handle handles an event.
func (d *Default) Handle(e event.Event) {
	if len(d.events) >= d.eventBuffer {
		<-d.events
	}
	d.events <- e
}

func (d *Default) Events() <-chan event.Event {
	return d.events
}
