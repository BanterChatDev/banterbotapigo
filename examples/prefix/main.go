package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	banter "github.com/BanterChatDev/banterbotapigo"
)

const token = "REPLACE_ME"

func main() {
	if token == "REPLACE_ME" {
		log.Fatal("paste your bot token into the `token` constant at the top of this file")
	}

	bot := banter.NewBot(banter.BotOpts{
		Intents:       banter.IntentsDefault() | banter.IntentMessageContent,
		CommandPrefix: "!",
		HelpEnabled:   true,
	})

	bot.PrefixCommand("ping", "Reply with pong and current time", "Info",
		func(ctx context.Context, c *banter.Context) error {
			return c.Reply(ctx, "pong "+time.Now().Format("15:04:05"))
		},
	)

	bot.PrefixCommand("about", "Show info about this bot", "Info",
		func(ctx context.Context, c *banter.Context) error {
			e := banter.NewEmbed().
				Title("About this bot").
				Description("Demo bot built with banterbotapigo.").
				ColorInt(0x5865f2).
				AddField("Go", runtime.Version(), true).
				AddField("OS/Arch", runtime.GOOS+"/"+runtime.GOARCH, true).
				AddField("SDK", banter.Version, true).
				Footer("type !help for command list", "")
			return c.ReplyEmbed(ctx, e)
		},
		"info", "whoami",
	)

	bot.PrefixCommand("say", "Echo back the rest of your message", "Fun",
		func(ctx context.Context, c *banter.Context) error {
			args := strings.TrimSpace(c.RawArgs)
			if args == "" {
				return c.Reply(ctx, "usage: !say <message>")
			}
			return c.Reply(ctx, fmt.Sprintf("you said: %s", args))
		},
	)

	if err := bot.Run(context.Background(), token); err != nil {
		log.Fatalf("bot exited: %s", err)
	}
}