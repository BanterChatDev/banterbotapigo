package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	banter "github.com/BanterChatDev/banterbotapigo"
)

const token = "REPLACE_ME"

func main() {
	if token == "REPLACE_ME" {
		log.Fatal("paste your bot token into the `token` constant at the top of this file")
	}

	bot := banter.NewBot(banter.BotOpts{
		Intents: banter.IntentsDefault() | banter.IntentMessageContent,
	})

	bot.SlashCommand("newchannel", "Create a text channel in this guild",
		[]banter.SlashOption{
			banter.NewSlashOption("name", banter.OptionString).Describe("channel name (lowercase, dashes ok)"),
		},
		func(ctx context.Context, i *banter.Interaction) error {
			if i.GuildID == "" {
				return i.Respond(ctx, "this command only works inside a guild.", banter.RespondOpts{Ephemeral: true})
			}
			name := strings.TrimSpace(i.OptString("name", ""))
			if name == "" {
				return i.Respond(ctx, "name cannot be empty.", banter.RespondOpts{Ephemeral: true})
			}

			ch, err := bot.CreateChannel(ctx, i.GuildID, name, banter.ChannelOpts{})
			if err != nil {
				return i.Respond(ctx,
					fmt.Sprintf("could not create channel: %s", err),
					banter.RespondOpts{Ephemeral: true},
				)
			}

			e := banter.NewEmbed().
				Title("Channel created").
				Description(fmt.Sprintf("**#%s** is ready.", ch.Name)).
				ColorInt(0x57F287).
				AddField("ID", ch.ID, true).
				AddField("Type", ch.Type, true)
			return i.Respond(ctx, "", banter.RespondOpts{Embed: e})
		},
	)

	if err := bot.Run(context.Background(), token); err != nil {
		log.Fatalf("bot exited: %s", err)
	}
}