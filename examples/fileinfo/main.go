package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

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
	})

	bot.PrefixCommand("fileinfo", "Reply with info about attached files", "Files",
		func(ctx context.Context, c *banter.Context) error {
			if len(c.Message.Attachments) == 0 {
				return c.Reply(ctx, "attach one or more files along with `!fileinfo` to inspect them.")
			}

			e := banter.NewEmbed().
				Title("File info").
				Description(fmt.Sprintf("Got %d attachment(s).", len(c.Message.Attachments))).
				ColorInt(0x5865f2)

			for _, a := range c.Message.Attachments {
				ext := strings.TrimPrefix(filepath.Ext(a.Filename), ".")
				if ext == "" {
					ext = "(none)"
				}
				e.AddField(
					a.Filename,
					fmt.Sprintf("ID: `%s`\nSize: %d bytes\nExt: %s", a.ID, a.Size, ext),
					false,
				)
			}

			return c.ReplyEmbed(ctx, e)
		},
	)

	if err := bot.Run(context.Background(), token); err != nil {
		log.Fatalf("bot exited: %s", err)
	}
}