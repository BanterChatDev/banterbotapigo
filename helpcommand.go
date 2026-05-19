package banter

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func registerHelp(b *Bot) {
	b.PrefixCommand("help", "Lists all commands.", "Info", func(ctx context.Context, c *Context) error {
		b.mu.RLock()
		var prefixCmds []*prefixCommand
		for _, cmd := range b.prefixCommands {
			if cmd.Name != "help" {
				prefixCmds = append(prefixCmds, cmd)
			}
		}
		slashCmds := append([]map[string]any{}, b.slashCommands...)
		b.mu.RUnlock()

		if len(prefixCmds) == 0 && len(slashCmds) == 0 {
			return c.Reply(ctx, "No commands registered.")
		}

		title := "Bot commands"
		if b.User != nil {
			title = fmt.Sprintf("%s commands", b.User.Username)
		}
		e := NewEmbed().Title(title).ColorInt(b.helpColor)

		prefixGroups := groupPrefixByCategory(prefixCmds)
		for _, label := range sortedKeys(prefixGroups) {
			body := formatPrefixLines(prefixGroups[label], b.commandPrefix)
			if body != "" {
				e.AddField(label, body, false)
			}
		}

		if len(slashCmds) > 0 {
			slashGroups := groupSlashByCategory(slashCmds)
			for _, label := range sortedKeys(slashGroups) {
				lines := formatSlashLines(slashGroups[label])
				if lines != "" {
					e.AddField(label, lines, false)
				}
			}
		}

		e.Footer(fmt.Sprintf("Prefix: %s · use %shelp anytime", b.commandPrefix, b.commandPrefix), "")
		return c.ReplyEmbed(ctx, e)
	})
}

func groupPrefixByCategory(cmds []*prefixCommand) map[string][]*prefixCommand {
	g := map[string][]*prefixCommand{}
	for _, cmd := range cmds {
		label := cmd.Category
		if label == "" {
			label = "Other"
		}
		g[label] = append(g[label], cmd)
	}
	return g
}

func groupSlashByCategory(cmds []map[string]any) map[string][]map[string]any {
	g := map[string][]map[string]any{}
	for _, c := range cmds {
		label, _ := c["_category"].(string)
		if label == "" {
			label = "Slash Commands"
		}
		g[label] = append(g[label], c)
	}
	return g
}

func sortedKeys(m any) []string {
	var keys []string
	switch typed := m.(type) {
	case map[string][]*prefixCommand:
		for k := range typed {
			keys = append(keys, k)
		}
	case map[string][]map[string]any:
		for k := range typed {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	idx := -1
	for i, k := range keys {
		if k == "Other" {
			idx = i
			break
		}
	}
	if idx >= 0 {
		keys = append(append([]string{}, keys[:idx]...), keys[idx+1:]...)
		keys = append(keys, "Other")
	}
	return keys
}

func formatPrefixLines(cmds []*prefixCommand, prefix string) string {
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	var lines []string
	for _, c := range cmds {
		sig := fmt.Sprintf("`%s%s`", prefix, c.Name)
		if c.Help != "" {
			doc := strings.SplitN(c.Help, "\n", 2)[0]
			sig += " — " + doc
		}
		lines = append(lines, sig)
	}
	return strings.Join(lines, "\n")
}

func formatSlashLines(cmds []map[string]any) string {
	sort.Slice(cmds, func(i, j int) bool {
		ni, _ := cmds[i]["name"].(string)
		nj, _ := cmds[j]["name"].(string)
		return ni < nj
	})
	var lines []string
	for _, c := range cmds {
		name, _ := c["name"].(string)
		desc, _ := c["description"].(string)
		line := fmt.Sprintf("`/%s`", name)
		if desc != "" && desc != name {
			line += " — " + desc
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}