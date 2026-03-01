package pico

import (
	"github.com/Swarup012/solo/pkg/bus"
	"github.com/Swarup012/solo/pkg/channels"
	"github.com/Swarup012/solo/pkg/config"
)

func init() {
	channels.RegisterFactory("pico", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewPicoChannel(cfg.Channels.Pico, b)
	})
}
