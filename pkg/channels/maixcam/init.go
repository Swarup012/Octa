package maixcam

import (
	"github.com/Swarup012/solo/pkg/bus"
	"github.com/Swarup012/solo/pkg/channels"
	"github.com/Swarup012/solo/pkg/config"
)

func init() {
	channels.RegisterFactory("maixcam", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewMaixCamChannel(cfg.Channels.MaixCam, b)
	})
}
