package main

import (
	"context"
	"log"
	"os"
	"time"

	banter "github.com/BanterChatDev/banterbotapigo"
)

const token = "REPLACE_ME"

func main() {
	if token == "REPLACE_ME" {
		log.Fatal("paste your bot token into the `token` constant at the top of this file")
	}

	logger := banter.NewLogger("listguilds")

	bot := banter.NewBot(banter.BotOpts{
		Intents: banter.IntentsDefault(),
	})

	bot.OnReady(func(ctx context.Context) {
		logger.Info("connected as %s — listing guilds", bot.User.Username)

		if len(bot.Guilds) == 0 {
			logger.Warn("bot is not in any guilds yet")
			os.Exit(0)
		}
		logger.Info("in %d guild(s):", len(bot.Guilds))

		for id, g := range bot.Guilds {
			logger.Info("guild %s — name=%q owner=%s", id, g.Name, g.OwnerID)

			chs, err := bot.ListChannels(ctx, id)
			if err != nil {
				logger.Error("  failed to list channels: %s", err)
				continue
			}
			for _, ch := range chs {
				logger.Info("  #%s — id=%s type=%s category=%s", ch.Name, ch.ID, ch.Type, ch.CategoryID)
			}
		}

		logger.Info("done. shutting down.")
		go func() {
			time.Sleep(time.Second)
			os.Exit(0)
		}()
	})

	if err := bot.Run(context.Background(), token); err != nil {
		log.Fatalf("bot exited: %s", err)
	}
}