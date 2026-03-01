package feishu

import (
	"github.com/Swarup012/solo/pkg/bus"
	"github.com/Swarup012/solo/pkg/channels"
	"github.com/Swarup012/solo/pkg/config"
)

func init() {
	channels.RegisterFactory("feishu", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewFeishuChannel(cfg.Channels.Feishu, b)
	})
}
