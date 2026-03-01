package discord

import (
	"github.com/Swarup012/solo/pkg/bus"
	"github.com/Swarup012/solo/pkg/channels"
	"github.com/Swarup012/solo/pkg/config"
)

func init() {
	channels.RegisterFactory("discord", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewDiscordChannel(cfg.Channels.Discord, b)
	})
}
