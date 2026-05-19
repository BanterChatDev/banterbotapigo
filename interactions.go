package banter

import (
	"context"
	"encoding/json"
	"errors"
)

type Interaction struct {
	ID          string
	Token       string
	AppID       string
	Type        string
	CommandName string
	CustomID    string
	MessageID   string
	Options     map[string]any
	GuildID     string
	ChannelID   string
	UserID      string

	client    *clientRef
	responded bool
}

func newInteractionFromJSON(raw json.RawMessage, client *clientRef) (*Interaction, error) {
	var p struct {
		ID              string         `json:"id"`
		Token           string         `json:"token"`
		AppID           string         `json:"app_id"`
		Type            string         `json:"type"`
		CommandName     string         `json:"command_name"`
		CustomID        string         `json:"custom_id"`
		MessageID       string         `json:"message_id"`
		SourceMessageID string         `json:"source_message_id"`
		Options         map[string]any `json:"options"`
		GuildID         string         `json:"guild_id"`
		ChannelID       string         `json:"channel_id"`
		UserID          string         `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	t := p.Type
	if t == "" {
		t = "slash"
	}
	msgID := p.MessageID
	if msgID == "" {
		msgID = p.SourceMessageID
	}
	opts := p.Options
	if opts == nil {
		opts = map[string]any{}
	}
	return &Interaction{
		ID:          p.ID,
		Token:       p.Token,
		AppID:       p.AppID,
		Type:        t,
		CommandName: p.CommandName,
		CustomID:    p.CustomID,
		MessageID:   msgID,
		Options:     opts,
		GuildID:     p.GuildID,
		ChannelID:   p.ChannelID,
		UserID:      p.UserID,
		client:      client,
	}, nil
}

func (i *Interaction) IsButton() bool { return i.Type == "button" }
func (i *Interaction) IsSlash() bool  { return i.Type == "slash" }

func (i *Interaction) OptString(name, fallback string) string {
	v, ok := i.Options[name]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	return s
}

func (i *Interaction) OptInt(name string, fallback int) int {
	v, ok := i.Options[name]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return fallback
}

func (i *Interaction) OptBool(name string, fallback bool) bool {
	v, ok := i.Options[name]
	if !ok {
		return fallback
	}
	b, ok := v.(bool)
	if !ok {
		return fallback
	}
	return b
}

type RespondOpts struct {
	Embed      *Embed
	Ephemeral  bool
	Components []map[string]any
	ReplyTo    string
}

func mergeComponents(embed *Embed, components []map[string]any) []map[string]any {
	if components != nil {
		return components
	}
	if embed != nil {
		return embed.PendingComponents()
	}
	return nil
}

func (i *Interaction) Respond(ctx context.Context, content string, opts RespondOpts) error {
	if i.client == nil {
		return errors.New("interaction has no attached client")
	}
	if content == "" && opts.Embed == nil {
		return errors.New("respond requires non-empty content or an embed")
	}
	if i.responded {
		return i.Followup(ctx, content, opts)
	}
	body := map[string]any{
		"kind":      "reply",
		"content":   content,
		"ephemeral": opts.Ephemeral,
	}
	if opts.Embed != nil {
		body["embed"] = opts.Embed.ToDict()
	}
	if comps := mergeComponents(opts.Embed, opts.Components); comps != nil {
		body["components"] = comps
	}
	if err := i.dispatch(ctx, body); err != nil {
		return err
	}
	i.responded = true
	return nil
}

func (i *Interaction) Defer(ctx context.Context, ephemeral bool) error {
	if i.client == nil {
		return errors.New("interaction has no attached client")
	}
	if i.responded {
		return errors.New("interaction already responded to")
	}
	body := map[string]any{
		"kind":      "defer",
		"ephemeral": ephemeral,
	}
	if err := i.dispatch(ctx, body); err != nil {
		return err
	}
	i.responded = true
	return nil
}

func (i *Interaction) Followup(ctx context.Context, content string, opts RespondOpts) error {
	if i.client == nil {
		return errors.New("interaction has no attached client")
	}
	if content == "" && opts.Embed == nil {
		return errors.New("followup requires non-empty content or an embed")
	}
	body := map[string]any{
		"kind":      "followup",
		"content":   content,
		"ephemeral": opts.Ephemeral,
	}
	if opts.Embed != nil {
		body["embed"] = opts.Embed.ToDict()
	}
	if comps := mergeComponents(opts.Embed, opts.Components); comps != nil {
		body["components"] = comps
	}
	if opts.ReplyTo != "" {
		body["reply_to"] = opts.ReplyTo
	}
	return i.dispatch(ctx, body)
}

func (i *Interaction) Update(ctx context.Context, content string, opts RespondOpts) error {
	if i.Type != "button" {
		return errors.New("update is only valid for button interactions — use Respond or Followup for slash commands")
	}
	if i.responded {
		return errors.New("interaction already responded to")
	}
	if i.client == nil {
		return errors.New("interaction has no attached client")
	}
	body := map[string]any{
		"kind":    "update",
		"content": content,
	}
	if opts.Embed != nil {
		body["embed"] = opts.Embed.ToDict()
	}
	if comps := mergeComponents(opts.Embed, opts.Components); comps != nil {
		body["components"] = comps
	}
	if err := i.dispatch(ctx, body); err != nil {
		return err
	}
	i.responded = true
	return nil
}

func (i *Interaction) dispatch(ctx context.Context, body map[string]any) error {
	_ = ctx
	_ = body
	return errors.New("interaction.dispatch not wired: HTTP client not implemented yet (chunk 3)")
}