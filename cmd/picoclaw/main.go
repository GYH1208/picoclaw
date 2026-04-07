// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/agent"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/auth"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cron"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/gateway"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/migrate"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/model"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/onboard"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/skills"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/status"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/version"
	"github.com/sipeed/picoclaw/pkg/config"
)

func NewPicoclawCommand() *cobra.Command {
	short := fmt.Sprintf("%s picoclaw - Personal AI Assistant v%s\n\n", internal.Logo, config.GetVersion())

	cmd := &cobra.Command{
		Use:     "picoclaw",
		Short:   short,
		Example: "picoclaw version",
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
		model.NewModelCommand(),
		version.NewVersionCommand(),
	)

	return cmd
}

const (
	banner = "\r\n" +
		"\033[1;38;2;62;93;185m" + " _____ ____ ___ _   _  _" + "\033[1;38;2;213;70;70m" + "___ ____   ___        __\n" +
		"\033[1;38;2;62;93;185m" + "|_   _/ ___|_ _| \\ | |/ " + "\033[1;38;2;213;70;70m" + "___|  _ \\ / \\ \\      / /\n" +
		"\033[1;38;2;62;93;185m" + "  | | \\___ \\| ||  \\| | |" + "\033[1;38;2;213;70;70m" + "  _| |_) / _ \\ \\ /\\ / / \n" +
		"\033[1;38;2;62;93;185m" + "  | |  ___) | || |\\  | |" + "\033[1;38;2;213;70;70m" + "_| |  __/ ___ \\ V  V /  \n" +
		"\033[1;38;2;62;93;185m" + "  |_| |____/___|_| \\_|\\_" + "\033[1;38;2;213;70;70m" + "___|_| /_/   \\_\\_/\\_/   \n" +
		"\033[0m\r\n"
)

func main() {
	fmt.Printf("%s", banner)
	cmd := NewPicoclawCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
