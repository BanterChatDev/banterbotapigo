package banter

import "fmt"

type Embed struct {
	data    map[string]any
	buttons []map[string]any
}

func NewEmbed() *Embed {
	return &Embed{
		data:    map[string]any{"fields": []map[string]any{}},
		buttons: nil,
	}
}

func (e *Embed) Title(s string) *Embed {
	if s != "" {
		e.data["title"] = s
	}
	return e
}

func (e *Embed) Description(s string) *Embed {
	if s != "" {
		e.data["description"] = s
	}
	return e
}

func (e *Embed) ColorInt(c int) *Embed {
	e.data["color"] = fmt.Sprintf("#%06x", c&0xffffff)
	return e
}

func (e *Embed) ColorHex(s string) *Embed {
	if s != "" {
		e.data["color"] = s
	}
	return e
}

func (e *Embed) URL(s string) *Embed {
	if s != "" {
		e.data["url"] = s
	}
	return e
}

func (e *Embed) Image(url string) *Embed {
	if url != "" {
		e.data["image"] = map[string]string{"url": url}
	}
	return e
}

func (e *Embed) Thumbnail(url string) *Embed {
	if url != "" {
		e.data["thumbnail"] = map[string]string{"url": url}
	}
	return e
}

func (e *Embed) AddField(name, value string, inline bool) *Embed {
	fields := e.data["fields"].([]map[string]any)
	e.data["fields"] = append(fields, map[string]any{
		"name":   name,
		"value":  value,
		"inline": inline,
	})
	return e
}

func (e *Embed) Footer(text, iconURL string) *Embed {
	f := map[string]string{"text": text}
	if iconURL != "" {
		f["icon_url"] = iconURL
	}
	e.data["footer"] = f
	return e
}

func (e *Embed) Author(name, url, iconURL string) *Embed {
	a := map[string]string{"name": name}
	if url != "" {
		a["url"] = url
	}
	if iconURL != "" {
		a["icon_url"] = iconURL
	}
	e.data["author"] = a
	return e
}

type ButtonOpts struct {
	Style    string
	URL      string
	Emoji    string
	Disabled bool
	CustomID string
}

func (e *Embed) AddButton(label string, opts ButtonOpts) *Embed {
	style := opts.Style
	if style == "" {
		style = "secondary"
	}
	btn := map[string]any{
		"type":  "button",
		"label": label,
		"style": style,
	}
	if opts.Emoji != "" {
		btn["emoji"] = opts.Emoji
	}
	if opts.Disabled {
		btn["disabled"] = true
	}
	if style == "link" {
		if opts.URL != "" {
			btn["url"] = opts.URL
		}
	} else if opts.CustomID != "" {
		btn["custom_id"] = opts.CustomID
	}
	e.buttons = append(e.buttons, btn)
	return e
}

func (e *Embed) PendingComponents() []map[string]any {
	if len(e.buttons) == 0 {
		return nil
	}
	rows := []map[string]any{}
	for i := 0; i < len(e.buttons); i += 5 {
		end := i + 5
		if end > len(e.buttons) {
			end = len(e.buttons)
		}
		rows = append(rows, map[string]any{
			"type":       "action_row",
			"components": e.buttons[i:end],
		})
		if len(rows) >= 5 {
			break
		}
	}
	return rows
}

func (e *Embed) ToDict() map[string]any {
	out := make(map[string]any, len(e.data))
	for k, v := range e.data {
		out[k] = v
	}
	if fields, ok := out["fields"].([]map[string]any); ok && len(fields) == 0 {
		delete(out, "fields")
	}
	return out
}