package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	banter "github.com/BanterChatDev/banterbotapigo"
)

const token = "bt_16a87725c907.2a25cbf2e8b3acc1e18ba502e84f83f9"

func main() {
	if token == "REPLACE_ME" {
		log.Fatal("paste your bot token into the `token` constant at the top of this file")
	}

	bot := banter.NewBot(banter.BotOpts{
		Intents: banter.IntentsDefault() | banter.IntentMessageContent,
	})

	startedAt := time.Now()

	bot.SlashCommand("ping", "Quick health check", nil,
		func(ctx context.Context, i *banter.Interaction) error {
			return i.Respond(ctx, "pong", banter.RespondOpts{})
		},
	)

	bot.SlashCommand("uptime", "How long the bot has been running", nil,
		func(ctx context.Context, i *banter.Interaction) error {
			d := time.Since(startedAt).Round(time.Second)
			return i.Respond(ctx, fmt.Sprintf("up for %s", d), banter.RespondOpts{Ephemeral: true})
		},
	)

	bot.SlashCommand("echo", "Repeat what you sent",
		[]banter.SlashOption{
			banter.NewSlashOption("text", banter.OptionString).Describe("what to repeat"),
			banter.NewSlashOption("loud", banter.OptionBoolean).Describe("uppercase?").Optional(),
		},
		func(ctx context.Context, i *banter.Interaction) error {
			text := i.OptString("text", "")
			if i.OptBool("loud", false) {
				text = strings.ToUpper(text)
			}
			return i.Respond(ctx, text, banter.RespondOpts{})
		},
	)

	if err := bot.Run(context.Background(), token); err != nil {
		log.Fatalf("bot exited: %s", err)
	}
}