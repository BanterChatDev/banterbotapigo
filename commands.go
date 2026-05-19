package banter

import (
	"context"
	"errors"
	"strings"
)

const (
	OptionString  = "STRING"
	OptionInteger = "INTEGER"
	OptionBoolean = "BOOLEAN"
	OptionUser    = "USER"
	OptionChannel = "CHANNEL"
	OptionRole    = "ROLE"
)

type SlashOption struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

func NewSlashOption(name, optType string) SlashOption {
	return SlashOption{Name: name, Type: optType, Required: true}
}

func (o SlashOption) Describe(s string) SlashOption {
	o.Description = s
	return o
}

func (o SlashOption) Optional() SlashOption {
	o.Required = false
	return o
}

func (o SlashOption) toDict() map[string]any {
	d := map[string]any{
		"name":     o.Name,
		"type":     o.Type,
		"required": o.Required,
	}
	if o.Description != "" {
		d["description"] = o.Description
	}
	return d
}

type CommandError struct{ Msg string }

func (e *CommandError) Error() string { return e.Msg }

type CommandNotFound struct{ CommandError }
type MissingArgument struct {
	CommandError
	Name string
}
type BadArgument struct {
	CommandError
	Name     string
	Value    string
	Expected string
}

type Context struct {
	Bot     *Bot
	Message *Message
	Name    string
	RawArgs string
}

func (c *Context) Reply(ctx context.Context, content string) error {
	if c.Bot == nil || c.Bot.http == nil {
		return errors.New("context detached from bot")
	}
	_, err := c.Bot.http.SendMessage(ctx, c.Message.ChannelID, SendMessageBody{
		Content: content,
		ReplyTo: c.Message.ID,
	})
	return err
}

func (c *Context) ReplyEmbed(ctx context.Context, embed *Embed) error {
	if c.Bot == nil || c.Bot.http == nil {
		return errors.New("context detached from bot")
	}
	body := SendMessageBody{
		Content: "",
		ReplyTo: c.Message.ID,
	}
	if embed != nil {
		body.Embed = embed.ToDict()
		if comps := embed.PendingComponents(); comps != nil {
			body.Components = comps
		}
	}
	_, err := c.Bot.http.SendMessage(ctx, c.Message.ChannelID, body)
	return err
}

type PrefixHandler func(ctx context.Context, c *Context) error

type prefixCommand struct {
	Name     string
	Aliases  []string
	Help     string
	Category string
	Handler  PrefixHandler
}

type SlashHandler func(ctx context.Context, i *Interaction) error
type ButtonHandler func(ctx context.Context, i *Interaction) error

func splitArgs(raw string) []string {
	return strings.Fields(raw)
}

func HasPermissions(required int64, handler SlashHandler) SlashHandler {
	return func(ctx context.Context, i *Interaction) error {
		_ = required
		return handler(ctx, i)
	}
}