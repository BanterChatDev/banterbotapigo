package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
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
	})

	bot.PrefixCommand("randomfile", "Generate a random text file and upload it", "Files",
		func(ctx context.Context, c *banter.Context) error {
			var lines []string
			for i := 0; i < 20; i++ {
				buf := make([]byte, 8)
				_, _ = rand.Read(buf)
				lines = append(lines, fmt.Sprintf("line %02d: %s", i+1, hex.EncodeToString(buf)))
			}
			body := strings.Join(lines, "\n") + "\n"

			filename := fmt.Sprintf("random-%d.txt", time.Now().Unix())
			f, err := banter.NewFileFromBytes([]byte(body), filename)
			if err != nil {
				return c.Reply(ctx, "could not build file: "+err.Error())
			}

			raw, err := bot.HTTP.UploadAttachment(ctx, c.Message.ChannelID, f)
			if err != nil {
				return c.Reply(ctx, "upload failed: "+err.Error())
			}

			var att struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &att); err != nil {
				return c.Reply(ctx, "could not parse upload response: "+err.Error())
			}

			_, err = bot.HTTP.SendMessage(ctx, c.Message.ChannelID, banter.SendMessageBody{
				Content:       fmt.Sprintf("here's your random file (%d bytes)", len(body)),
				AttachmentIDs: []string{att.ID},
				ReplyTo:       c.Message.ID,
			})
			return err
		},
	)

	if err := bot.Run(context.Background(), token); err != nil {
		log.Fatalf("bot exited: %s", err)
	}
}