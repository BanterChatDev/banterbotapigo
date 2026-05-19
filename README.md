# banterbotapigo

This is the Go SDK for [Banter](https://banterchat.org), a one-to-one port of the Python [banterbotapi](https://pypi.org/project/banterbotapi/) library. If you have used `discord.py` or `banterbotapi` before, the patterns here will feel familiar. The goal is to let you write a Banter bot in Go with the same ergonomics a Python developer gets from those libraries, without the runtime overhead or the dependency on the Python interpreter.

## Installation

```bash
go get github.com/BanterChatDev/banterbotapigo
```

The library lives under one Go module at the repository root. There is no separate install step beyond `go get` and no compiled binary to fetch. Go pulls the source on demand and links it into your bot when you build.

## A first bot

The smallest useful bot is a handful of lines. It connects to the gateway, registers a single prefix command, and replies when the command fires. Everything else is optional.

```go
package main

import (
	"context"
	"log"

	banter "github.com/BanterChatDev/banterbotapigo"
)

func main() {
	bot := banter.NewBot(banter.BotOpts{
		Intents: banter.IntentsDefault() | banter.IntentMessageContent,
	})

	bot.PrefixCommand("ping", "Quick health check", "Info",
		func(ctx context.Context, c *banter.Context) error {
			return c.Reply(ctx, "pong")
		},
	)

	if err := bot.Run(context.Background(), "YOUR_BOT_TOKEN"); err != nil {
		log.Fatal(err)
	}
}
```

The `IntentMessageContent` flag is needed for the bot to actually see the text of messages, which prefix commands depend on. Without it the gateway will deliver `message_create` events but the `Content` field will be empty.

## Slash commands

Slash commands are registered with `Bot.SlashCommand` and respond through the `Interaction` argument. The signature gives you a typed handler and a structured options list. Options have a builder pattern for type, description, and required/optional state.

```go
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
```

Registering the same slash command name twice is treated as a programmer error. The library will log a critical message and exit the process. This is deliberate. It catches a class of bug where two command loaders end up registering the same name from different files, which would otherwise silently overwrite one handler and create a confusing runtime mystery. The same applies to prefix commands and to exact-match button handlers.

The interaction can be answered in a few different ways. The most common is `Respond`, which posts a visible reply. `Defer` acknowledges the interaction within the three-second window and lets you reply later via `Followup`, which is useful for handlers that take longer to compute. `Update` is specific to button interactions and edits the source message in place. All four methods take a `RespondOpts` struct where you can attach an embed, mark the reply ephemeral, or pass component rows.

## Embeds

Embeds use a fluent builder so chains read top to bottom in the order they appear in the rendered message.

```go
e := banter.NewEmbed().
	Title("Status").
	Description("All systems nominal.").
	ColorInt(0x57F287).
	AddField("Uptime", "12d 4h", true).
	AddField("Users", "1,204", true).
	Footer("Last checked just now", "")
```

Buttons attach to embeds and travel with them through any method that sends a message — `Bot.SendEmbed`, `Context.ReplyEmbed`, and `Interaction.Respond` with an `Embed` set in `RespondOpts`. Each button has a label, a style, and either a URL (for link buttons) or a custom ID (for handler buttons). Up to five buttons fit in a row and up to five rows fit on a message.

```go
e.AddButton("Confirm", banter.ButtonOpts{Style: "success", CustomID: "confirm_yes"})
e.AddButton("Cancel", banter.ButtonOpts{Style: "danger", CustomID: "confirm_no"})

bot.OnButton("confirm_yes", func(ctx context.Context, i *banter.Interaction) error {
	return i.Update(ctx, "Confirmed.", banter.RespondOpts{})
})
```

Button handlers can match exactly or by prefix. Passing `"page_*"` to `OnButton` registers a glob that fires for any custom ID starting with `page_`. This is useful for pagination where the page number is embedded in the ID. Reading the suffix from `Interaction.CustomID` inside the handler lets one function handle every page click.

## The help command

A help command is registered automatically. Typing `!help` (or whatever prefix you set) replies with an embed listing every prefix and slash command grouped by category. This matches the behavior of `discord.py` and the Python SDK.

If you want a different help command you have three options, and the API is designed to express them clearly.

The default is to omit the option entirely. The library registers its own help command and you get it for free.

```go
bot := banter.NewBot(banter.BotOpts{Intents: banter.IntentsDefault()})
```

To disable the help command, pass `NoHelpCommand`. Nothing is registered under that name and `!help` does nothing.

```go
bot := banter.NewBot(banter.BotOpts{
	Intents:     banter.IntentsDefault(),
	HelpCommand: banter.NoHelpCommand,
})
```

To replace it with your own, wrap a handler in `CustomHelpCommand`. Your handler is registered as `!help` and takes priority over the default.

```go
bot := banter.NewBot(banter.BotOpts{
	Intents: banter.IntentsDefault(),
	HelpCommand: banter.CustomHelpCommand(func(ctx context.Context, c *banter.Context) error {
		return c.Reply(ctx, "use the docs at example.com/help")
	}),
})
```

Whichever option you pick, the dedup behavior still applies. If you both pass a `CustomHelpCommand` and call `bot.PrefixCommand("help", ...)` yourself, the second registration panics. This catches the case where you forgot you already provided a custom help and would otherwise quietly overwrite it.

## Events

The bot has typed methods for the events users typically care about: `OnMessage`, `OnReady`, `OnMessageEdit`, `OnMessageDelete`, `OnMemberJoin`, `OnReactionAdd`, `OnReactionRemove`, `OnResumed`, and `OnError`. Each accepts a function with the right signature for that event, so you get autocompletion and the compiler catches a wrong argument type before you ship.

```go
bot.OnReady(func(ctx context.Context) {
	log.Printf("bot ready as %s", bot.User.Username)
})

bot.OnMessage(func(ctx context.Context, m *banter.Message) {
	if m.Content == "ping" {
		bot.SendMessage(ctx, m.ChannelID, "pong")
	}
})
```

Every handler runs in its own goroutine. This means concurrent messages fire concurrent handlers, which is the right default for a chat bot but worth remembering when you write to shared state. Use a mutex or a channel-based design if your handlers touch the same map or counter.

If a handler returns an error or panics, the library does not crash. Panics are recovered and logged. Errors returned from slash and prefix handlers are routed to your `OnError` callback. The point is to keep the bot alive even when individual handlers misbehave.

## HTTP access

The Bot type wraps the most common operations as convenience methods that return typed values. `SendMessage`, `SendEmbed`, `CreateChannel`, `CreateCategory`, `ListChannels`, `GetChannel`, and `GetGuild` all unmarshal the response into the appropriate model and attach a back-reference to the bot so the returned value can be used as the receiver of further calls.

For anything the convenience layer does not cover, the underlying HTTP client is exposed as `bot.HTTP`. Every endpoint described in the Banter API is wrapped here as a method. The return value is a raw `[]byte` so you can unmarshal into whatever shape you want.

```go
raw, err := bot.HTTP.KickMember(ctx, guildID, userID)
```

This split keeps the convenience layer small and predictable without forcing you to write a wrapper every time you reach for a less common endpoint. The same applies to `UploadAttachment`, which takes a `*File` built from `NewFileFromPath`, `NewFileFromBytes`, or `NewFileFromReader` and uploads it as multipart form data.

## Errors

The library uses Go's standard error interface with a hierarchy that lets you handle specific cases. The base type is `BanterError`. HTTP failures wrap it as `HTTPException`, and specific HTTP statuses get their own types — `Forbidden`, `NotFound`, `RateLimited`, `LoginFailure`, and `DuplicateCommand`. Use `errors.As` to discriminate.

```go
var rl *banter.RateLimited
if errors.As(err, &rl) {
	time.Sleep(time.Duration(rl.RetryAfter * float64(time.Second)))
}
```

For rate limits, the HTTP client already retries once automatically based on the `X-RateLimit-Reset-After` header. The `RateLimited` error only surfaces if both attempts fail, so by the time you see it the simple wait-and-retry has already been tried.

Sentinel errors live in `errors.go` for the conditions that don't carry extra data — `ErrInteractionDetached`, `ErrInteractionResponded`, `ErrEveryoneNotFound`, and so on. Use `errors.Is` to check for these.

## The gateway

The connection to the Banter gateway is managed automatically. The library handles the WebSocket handshake, heartbeats, sequence tracking, and resume-on-reconnect. When the connection drops, it reconnects with exponential backoff and jitter, and if the session is still resumable it sends the resume payload so events from the gap can be replayed. If the close code indicates an unrecoverable failure — invalid token, banned, or a protocol violation — the bot stops cleanly rather than retrying forever.

You do not normally need to think about any of this. The default behavior is what almost every bot wants. If you do want to opt out of reconnect for testing, set `Reconnect: false` in `BotOpts`. The bot will then run until the first disconnect and exit.

## Logging

The library uses a coloured logger that writes to standard error. Levels are `Debug`, `Info`, `Warn`, `Error`, and `Panic`. The default level is `Info`. Set the `BANTERAPI_DEBUG` environment variable to `1`, `true`, or `yes` to switch to `Debug` and see the per-request HTTP and gateway frame logs. Set `NO_COLOR` to disable ANSI escapes — useful in environments that pipe the output to a file or a logging service that does not handle the escapes.

You can use the same logger in your own code by calling `banter.NewLogger("yourname")`. The name appears as the third column of each log line and helps separate your messages from the library's. There is no separate user-facing logger and no second formatting pass to maintain.

## Pagination

The `Paginator` type packages the common case where you want to show a list of items across multiple pages with Back and Forward buttons. You give it a fetch callback that returns the full slice, a per-page count, and a build callback that renders a slice into an embed. The paginator handles slicing, button wiring, and the page-number-in-custom-id round trip.

The full pagination example lives under `examples/` and is the easiest way to understand the flow end to end. The short version is that you build the paginator once, respond to the initial slash command with `paginator.Respond`, and route button clicks for the matching prefix to `paginator.Update`.

## Examples

The repository contains six examples under `examples/`, each in its own folder with a single `main.go`. They cover slash commands with options, prefix commands with aliases and an auto-help integration, channel creation, walking the bot's guild list with the library logger, generating and uploading a random text file, and inspecting attached files on a message. Together they touch most of the public surface and serve as the fastest way to learn the library after this README.

Run any of them from the repository root with `go run ./examples/<name>`. Each `main.go` has a `const token = "REPLACE_ME"` at the top that you swap with your real bot token before running. Be careful to revert the token to `REPLACE_ME` before pushing, since the whitelist `.gitignore` tracks every `.go` file under `examples/`.

## Compatibility

The library targets Go 1.22 and later. It depends on `github.com/gorilla/websocket` for the gateway connection and on the standard library for everything else. The module path is `github.com/BanterChatDev/banterbotapigo` and the import alias is `banter` by convention.

## Status

The library is at version 0.1.0 and is functionally complete relative to the Python SDK it ports. Both libraries cover the same set of HTTP endpoints, the same event types, the same gateway behavior including resume on reconnect, and the same dedup-on-duplicate-command pattern. There are minor differences in how the API surfaces in each language because Go and Python have different idioms for kwargs, decorators, and async, but anything you can do in one library you can do in the other.

The first tagged release will go out as `v0.1.0` once the examples have run end to end against a live test bot. After that, semantic versioning applies. The public API surface as documented here is the contract.

## License

MIT. See `LICENSE` for the full text.