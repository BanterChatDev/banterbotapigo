package banter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

var pagLog = newLogger("pagination")

type BuildEmbedFn func(items []any, page, total int) *Embed
type FetchFn func() []any

type Paginator struct {
	Fetch         FetchFn
	PerPage       int
	BuildEmbed    BuildEmbedFn
	Prefix        string
	EmptyMessage  string
}

func NewPaginator(fetch FetchFn, perPage int, build BuildEmbedFn) *Paginator {
	if perPage < 1 {
		perPage = 1
	}
	return &Paginator{
		Fetch:        fetch,
		PerPage:      perPage,
		BuildEmbed:   build,
		Prefix:       "page",
		EmptyMessage: "Nothing to show.",
	}
}

func (p *Paginator) slice(page int) ([]any, int, int, []any) {
	items := p.Fetch()
	if items == nil {
		items = []any{}
	}
	if len(items) == 0 {
		return nil, 0, 1, items
	}
	total := (len(items)-1)/p.PerPage + 1
	if total < 1 {
		total = 1
	}
	if page < 0 {
		page = 0
	}
	if page > total-1 {
		page = total - 1
	}
	start := page * p.PerPage
	end := start + p.PerPage
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], page, total, items
}

func (p *Paginator) embedFor(page int) *Embed {
	chunk, page, total, items := p.slice(page)
	pagLog.Debug("paginator slice prefix=%s page=%d total=%d items=%d", p.Prefix, page, total, len(items))
	if len(items) == 0 {
		return NewEmbed().Description(p.EmptyMessage).ColorInt(0x2B2D31)
	}
	embed := p.BuildEmbed(chunk, page, total)
	if total > 1 {
		embed.AddButton("Back", ButtonOpts{
			Style:    "secondary",
			CustomID: fmt.Sprintf("%s_%d", p.Prefix, page-1),
			Disabled: page <= 0,
		})
		embed.AddButton("Forward", ButtonOpts{
			Style:    "secondary",
			CustomID: fmt.Sprintf("%s_%d", p.Prefix, page+1),
			Disabled: page+1 >= total,
		})
	}
	return embed
}

func (p *Paginator) Respond(ctx context.Context, i *Interaction, page int, ephemeral bool) error {
	pagLog.Info("paginator respond prefix=%s page=%d user=%s", p.Prefix, page, i.UserID)
	return i.Respond(ctx, "", RespondOpts{Embed: p.embedFor(page), Ephemeral: ephemeral})
}

func (p *Paginator) Update(ctx context.Context, i *Interaction) error {
	idx := strings.LastIndex(i.CustomID, "_")
	if idx < 0 {
		pagLog.Info("paginator update could not parse page from custom_id=%q — deferring", i.CustomID)
		return i.Defer(ctx, false)
	}
	page, err := strconv.Atoi(i.CustomID[idx+1:])
	if err != nil {
		pagLog.Info("paginator update could not parse page from custom_id=%q — deferring", i.CustomID)
		return i.Defer(ctx, false)
	}
	pagLog.Info("paginator update prefix=%s custom_id=%s page=%d user=%s msg=%s",
		p.Prefix, i.CustomID, page, i.UserID, i.MessageID)
	return i.Update(ctx, "", RespondOpts{Embed: p.embedFor(page)})
}