// Octa - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Octa contributors

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Swarup012/solo/cmd/octa/internal"
	"github.com/Swarup012/solo/cmd/octa/internal/agent"
	"github.com/Swarup012/solo/cmd/octa/internal/auth"
	"github.com/Swarup012/solo/cmd/octa/internal/cron"
	"github.com/Swarup012/solo/cmd/octa/internal/gateway"
	"github.com/Swarup012/solo/cmd/octa/internal/migrate"
	"github.com/Swarup012/solo/cmd/octa/internal/onboard"
	"github.com/Swarup012/solo/cmd/octa/internal/skills"
	"github.com/Swarup012/solo/cmd/octa/internal/status"
	"github.com/Swarup012/solo/cmd/octa/internal/version"
)

func NewPicoclawCommand() *cobra.Command {
	short := fmt.Sprintf("%s octa - Personal AI Assistant v%s\n\n", internal.Logo, internal.GetVersion())

	cmd := &cobra.Command{
		Use:     "octa",
		Short:   short,
		Example: "octa agent",
	}

	cmd.AddCommand(
		onboard.NewOnboardCommand(),
		agent.NewAgentCommand(),
		auth.NewAuthCommand(),
		gateway.NewGatewayCommand(),
		status.NewStatusCommand(),
		cron.NewCronCommand(),
		migrate.NewMigrateCommand(),
		skills.NewSkillsCommand(),
		version.NewVersionCommand(),
	)

	return cmd
}

func main() {
	cmd := NewPicoclawCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
