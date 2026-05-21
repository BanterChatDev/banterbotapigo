package banter

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var botLog = NewLogger(loggerPrefix + "bot")

type BotOpts struct {
	Intents         int64
	BaseURL         string
	WSURL           string
	Reconnect       bool
	CommandPrefix   string
	ApplicationID   string
	HelpCommand     *HelpCommand
	HelpColor       int
	Debug           bool
}

type HelpCommand struct {
	disabled bool
	handler  PrefixHandler
}

var NoHelpCommand = &HelpCommand{disabled: true}

func DefaultHelpCommand() *HelpCommand {
	return &HelpCommand{}
}

func CustomHelpCommand(handler PrefixHandler) *HelpCommand {
	return &HelpCommand{handler: handler}
}

type MessageHandler func(ctx context.Context, m *Message)
type ReadyHandler func(ctx context.Context)
type ResumedHandler func(ctx context.Context)
type MessageEditHandler func(ctx context.Context, payload json.RawMessage)
type MessageDeleteHandler func(ctx context.Context, payload json.RawMessage)
type MemberJoinHandler func(ctx context.Context, m *Member)
type ReactionAddHandler func(ctx context.Context, payload json.RawMessage)
type ReactionRemoveHandler func(ctx context.Context, payload json.RawMessage)
type ErrorHandler func(ctx context.Context, err error)

type Bot struct {
	opts          BotOpts
	intents       int64
	baseURL       string
	wsURL         string
	reconnect     bool
	commandPrefix string
	applicationID string
	helpColor     int

	User      *User
	SessionID string
	Guilds    map[string]*Guild

	HTTP    *HTTPClient
	gateway *Gateway
	cref    *clientRef

	mu sync.RWMutex

	onMessage        []MessageHandler
	onReady          []ReadyHandler
	onResumed        []ResumedHandler
	onMessageEdit    []MessageEditHandler
	onMessageDelete  []MessageDeleteHandler
	onMemberJoin     []MemberJoinHandler
	onReactionAdd    []ReactionAddHandler
	onReactionRemove []ReactionRemoveHandler
	onError          []ErrorHandler

	prefixCommands    map[string]*prefixCommand
	prefixAliases     map[string]string
	slashCommands     []map[string]any
	slashHandlers     map[string]SlashHandler
	buttonExact       map[string]ButtonHandler
	buttonPrefix      []buttonPrefixEntry

	done     chan struct{}
	terminal bool
}

type buttonPrefixEntry struct {
	Prefix  string
	Handler ButtonHandler
}

func NewBot(opts BotOpts) *Bot {
	if opts.Intents == 0 {
		opts.Intents = IntentsDefault()
	}
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseURL
	}
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.WSURL == "" {
		ws := opts.BaseURL
		ws = strings.Replace(ws, "https://", "wss://", 1)
		ws = strings.Replace(ws, "http://", "ws://", 1)
		opts.WSURL = ws + DefaultGatewayPath
	}
	if opts.CommandPrefix == "" {
		opts.CommandPrefix = DefaultCommandPrefix
	}
	if opts.HelpColor == 0 {
		opts.HelpColor = DefaultHelpColorInt
	}

	b := &Bot{
		opts:           opts,
		intents:        opts.Intents,
		baseURL:        opts.BaseURL,
		wsURL:          opts.WSURL,
		reconnect:      opts.Reconnect,
		commandPrefix:  opts.CommandPrefix,
		applicationID:  opts.ApplicationID,
		helpColor:      opts.HelpColor,
		Guilds:         map[string]*Guild{},
		prefixCommands: map[string]*prefixCommand{},
		prefixAliases:  map[string]string{},
		slashCommands:  []map[string]any{},
		slashHandlers:  map[string]SlashHandler{},
		buttonExact:    map[string]ButtonHandler{},
	}
	if opts.Debug {
		setLogLevel(logDebug)
	}
	if opts.Reconnect == false {
		b.reconnect = false
	} else {
		b.reconnect = true
	}
	switch {
	case opts.HelpCommand == nil:
		registerHelp(b)
	case opts.HelpCommand.disabled:
	case opts.HelpCommand.handler != nil:
		b.PrefixCommand("help", "Lists all commands.", "Info", opts.HelpCommand.handler)
	default:
		registerHelp(b)
	}
	return b
}

func (b *Bot) OnMessage(h MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onMessage = append(b.onMessage, h)
}

func (b *Bot) OnReady(h ReadyHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onReady = append(b.onReady, h)
}

func (b *Bot) OnResumed(h ResumedHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onResumed = append(b.onResumed, h)
}

func (b *Bot) OnMessageEdit(h MessageEditHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onMessageEdit = append(b.onMessageEdit, h)
}

func (b *Bot) OnMessageDelete(h MessageDeleteHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onMessageDelete = append(b.onMessageDelete, h)
}

func (b *Bot) OnMemberJoin(h MemberJoinHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onMemberJoin = append(b.onMemberJoin, h)
}

func (b *Bot) OnReactionAdd(h ReactionAddHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onReactionAdd = append(b.onReactionAdd, h)
}

func (b *Bot) OnReactionRemove(h ReactionRemoveHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onReactionRemove = append(b.onReactionRemove, h)
}

func (b *Bot) OnError(h ErrorHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onError = append(b.onError, h)
}

func (b *Bot) PrefixCommand(name, help, category string, handler PrefixHandler, aliases ...string) {
	b.mu.Lock()
	if _, exists := b.prefixCommands[name]; exists {
		b.mu.Unlock()
		botLog.Panic(MsgDupPrefixCommand, name)
	}
	for _, a := range aliases {
		if _, exists := b.prefixAliases[a]; exists {
			b.mu.Unlock()
			botLog.Panic(MsgDupPrefixCommand, a)
		}
	}
	defer b.mu.Unlock()
	cmd := &prefixCommand{
		Name:     name,
		Aliases:  aliases,
		Help:     help,
		Category: category,
		Handler:  handler,
	}
	b.prefixCommands[name] = cmd
	for _, a := range aliases {
		b.prefixAliases[a] = name
	}
}

func (b *Bot) SlashCommand(name, description string, options []SlashOption, handler SlashHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.slashHandlers[name]; exists {
		b.mu.Unlock()
		botLog.Panic(MsgDupSlashCommand, name)
	}
	entry := map[string]any{
		"name":        name,
		"description": description,
	}
	if description == "" {
		entry["description"] = name
	}
	if len(options) > 0 {
		opts := make([]map[string]any, len(options))
		for i, o := range options {
			opts[i] = o.toDict()
		}
		entry["options"] = opts
	}
	b.slashCommands = append(b.slashCommands, entry)
	b.slashHandlers[name] = handler
}

func (b *Bot) OnButton(customID string, handler ButtonHandler) {
	if customID == "" {
		botLog.Panic(MsgEmptyButtonID)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if strings.HasSuffix(customID, "*") {
		prefix := strings.TrimSuffix(customID, "*")
		b.buttonPrefix = append(b.buttonPrefix, buttonPrefixEntry{Prefix: prefix, Handler: handler})
		return
	}
	if _, exists := b.buttonExact[customID]; exists {
		botLog.Panic(MsgDupButtonHandler, customID)
	}
	b.buttonExact[customID] = handler
}

func (b *Bot) pickButtonHandler(customID string) ButtonHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if h, ok := b.buttonExact[customID]; ok {
		return h
	}
	var best ButtonHandler
	bestLen := -1
	for _, e := range b.buttonPrefix {
		if strings.HasPrefix(customID, e.Prefix) && len(e.Prefix) > bestLen {
			best = e.Handler
			bestLen = len(e.Prefix)
		}
	}
	return best
}

func (b *Bot) syncCommands(ctx context.Context) error {
	b.mu.RLock()
	if len(b.slashCommands) == 0 {
		b.mu.RUnlock()
		return nil
	}
	wire := make([]map[string]any, 0, len(b.slashCommands))
	for _, entry := range b.slashCommands {
		clean := map[string]any{}
		for k, v := range entry {
			if !strings.HasPrefix(k, "_") {
				clean[k] = v
			}
		}
		wire = append(wire, clean)
	}
	b.mu.RUnlock()
	_, err := b.HTTP.RegisterCommands(ctx, wire)
	return err
}

func (b *Bot) processPrefixCommand(ctx context.Context, m *Message) {
	if b.User != nil && m.UserID == b.User.ID {
		return
	}
	content := m.Content
	if !strings.HasPrefix(content, b.commandPrefix) {
		return
	}
	stripped := content[len(b.commandPrefix):]
	if stripped == "" {
		return
	}
	parts := strings.SplitN(stripped, " ", 2)
	name := parts[0]
	rawArgs := ""
	if len(parts) > 1 {
		rawArgs = parts[1]
	}

	b.mu.RLock()
	cmd, ok := b.prefixCommands[name]
	if !ok {
		if aliasOf, hasAlias := b.prefixAliases[name]; hasAlias {
			cmd, ok = b.prefixCommands[aliasOf]
		}
	}
	b.mu.RUnlock()
	if !ok || cmd == nil {
		return
	}
	c := &Context{
		Bot:     b,
		Message: m,
		Name:    name,
		RawArgs: rawArgs,
	}
	if err := cmd.Handler(ctx, c); err != nil {
		b.fireError(ctx, err)
	}
}

func (b *Bot) fire(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				botLog.Error("handler panic in %s: %v", name, r)
			}
		}()
		fn()
	}()
}

func (b *Bot) fireError(ctx context.Context, err error) {
	b.mu.RLock()
	handlers := append([]ErrorHandler{}, b.onError...)
	b.mu.RUnlock()
	for _, h := range handlers {
		h := h
		b.fire("on_error", func() { h(ctx, err) })
	}
}

func (b *Bot) dispatch(ctx context.Context, eventType string, payload json.RawMessage) {
	switch eventType {
	case "ready":
		b.handleReady(ctx, payload)
	case "resumed":
		b.handleResumed(ctx)
	case "channel_message", "message_create":
		b.handleMessageCreate(ctx, payload)
	case "message_edit":
		b.mu.RLock()
		handlers := append([]MessageEditHandler{}, b.onMessageEdit...)
		b.mu.RUnlock()
		for _, h := range handlers {
			h := h
			b.fire("on_message_edit", func() { h(ctx, payload) })
		}
	case "message_delete":
		b.mu.RLock()
		handlers := append([]MessageDeleteHandler{}, b.onMessageDelete...)
		b.mu.RUnlock()
		for _, h := range handlers {
			h := h
			b.fire("on_message_delete", func() { h(ctx, payload) })
		}
	case "guild_member_add":
		var mem Member
		if err := json.Unmarshal(payload, &mem); err != nil {
			botLog.Info("guild_member_add unmarshal failed: %s", err)
			return
		}
		mem.client = b.cref
		b.mu.RLock()
		handlers := append([]MemberJoinHandler{}, b.onMemberJoin...)
		b.mu.RUnlock()
		for _, h := range handlers {
			h := h
			b.fire("on_member_join", func() { h(ctx, &mem) })
		}
	case "reaction_add":
		b.mu.RLock()
		handlers := append([]ReactionAddHandler{}, b.onReactionAdd...)
		b.mu.RUnlock()
		for _, h := range handlers {
			h := h
			b.fire("on_reaction_add", func() { h(ctx, payload) })
		}
	case "reaction_remove":
		b.mu.RLock()
		handlers := append([]ReactionRemoveHandler{}, b.onReactionRemove...)
		b.mu.RUnlock()
		for _, h := range handlers {
			h := h
			b.fire("on_reaction_remove", func() { h(ctx, payload) })
		}
	case "interaction_create":
		b.handleInteraction(ctx, payload)
	default:
		botLog.Debug("unhandled event: %s", eventType)
	}
}

func (b *Bot) handleReady(ctx context.Context, payload json.RawMessage) {
	var d struct {
		User          json.RawMessage `json:"user"`
		Guilds        []json.RawMessage `json:"guilds"`
		ApplicationID string          `json:"application_id"`
		SessionID     string          `json:"session_id"`
	}
	if err := json.Unmarshal(payload, &d); err != nil {
		botLog.Info("ready unmarshal failed: %s", err)
		return
	}
	if len(d.User) > 0 {
		var u User
		if err := json.Unmarshal(d.User, &u); err == nil {
			b.User = &u
		}
	}
	for _, gRaw := range d.Guilds {
		var g Guild
		if err := json.Unmarshal(gRaw, &g); err == nil {
			g.client = b.cref
			b.mu.Lock()
			b.Guilds[g.ID] = &g
			b.mu.Unlock()
		}
	}
	if d.ApplicationID != "" && b.applicationID == "" {
		b.applicationID = d.ApplicationID
	}
	b.SessionID = d.SessionID

	uname := "(unknown)"
	if b.User != nil {
		uname = b.User.Username
	}
	botLog.Info("gateway connected session_id=%s user=%s", b.SessionID, uname)

	if len(b.slashCommands) > 0 {
		if err := b.syncCommands(ctx); err != nil {
			var dup *DuplicateCommand
			if errors.As(err, &dup) {
				botLog.Info(MsgCommandSyncDup, dup.Name)
			} else {
				botLog.Info("command sync failed: %s", err)
			}
		}
	}

	b.mu.RLock()
	handlers := append([]ReadyHandler{}, b.onReady...)
	b.mu.RUnlock()
	for _, h := range handlers {
		h := h
		b.fire("on_ready", func() { h(ctx) })
	}
}

func (b *Bot) handleResumed(ctx context.Context) {
	botLog.Info("gateway resumed session_id=%s last_seq=%d", b.SessionID, b.gateway.LastSeq())
	b.mu.RLock()
	handlers := append([]ResumedHandler{}, b.onResumed...)
	b.mu.RUnlock()
	for _, h := range handlers {
		h := h
		b.fire("on_resumed", func() { h(ctx) })
	}
}

func (b *Bot) handleMessageCreate(ctx context.Context, payload json.RawMessage) {
	var m Message
	if err := json.Unmarshal(payload, &m); err != nil {
		botLog.Info("message_create unmarshal failed: %s", err)
		return
	}
	m.client = b.cref

	b.mu.RLock()
	handlers := append([]MessageHandler{}, b.onMessage...)
	b.mu.RUnlock()
	for _, h := range handlers {
		h := h
		b.fire("on_message", func() { h(ctx, &m) })
	}
	b.processPrefixCommand(ctx, &m)
}

func (b *Bot) autoErrorReply(ctx context.Context, in *Interaction) {
	opts := RespondOpts{Ephemeral: true}
	var err error
	if in.IsButton() {
		err = in.Update(ctx, MsgSlashHandlerFailed, opts)
	} else {
		err = in.Respond(ctx, MsgSlashHandlerFailed, opts)
	}
	if err != nil {
		botLog.Debug("auto error reply failed: %s", err)
	}
}

func (b *Bot) handleInteraction(ctx context.Context, payload json.RawMessage) {
	in, err := newInteractionFromJSON(payload, b.cref)
	if err != nil {
		botLog.Info("interaction unmarshal failed: %s", err)
		return
	}
	if in.IsSlash() {
		b.mu.RLock()
		h, ok := b.slashHandlers[in.CommandName]
		b.mu.RUnlock()
		if !ok {
			botLog.Debug("no handler for slash command %q", in.CommandName)
			return
		}
		b.fire("slash:"+in.CommandName, func() {
			if err := h(ctx, in); err != nil {
				b.autoErrorReply(ctx, in)
				b.fireError(ctx, err)
			}
		})
		return
	}
	if in.IsButton() {
		h := b.pickButtonHandler(in.CustomID)
		if h == nil {
			botLog.Debug("no handler for button %q", in.CustomID)
			return
		}
		b.fire("button:"+in.CustomID, func() {
			if err := h(ctx, in); err != nil {
				b.autoErrorReply(ctx, in)
				b.fireError(ctx, err)
			}
		})
		return
	}
	botLog.Debug("unknown interaction type: %s", in.Type)
}

func (b *Bot) Run(ctx context.Context, token string) error {
	if b.done != nil {
		return errors.New("banter: bot already running")
	}
	b.HTTP = NewHTTPClient(token, b.baseURL)
	b.cref = &clientRef{http: b.HTTP}
	b.done = make(chan struct{})

	go b.supervise(ctx, token)
	return nil
}

func (b *Bot) Done() <-chan struct{} {
	return b.done
}

func (b *Bot) supervise(ctx context.Context, token string) {
	defer close(b.done)
	defer b.HTTP.Close()

	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					botLog.Error("supervisor: gateway loop panic: %v\n%s", r, debug.Stack())
				}
			}()
			b.gatewayLoop(ctx, token)
		}()

		if ctx.Err() != nil {
			return
		}
		if b.terminal {
			return
		}

		botLog.Info("supervisor: gateway loop returned, restarting in 5s")
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func (b *Bot) gatewayLoop(ctx context.Context, token string) {
	attempts := 0
	var resume *resumeState

	for {
		connectedAt := time.Time{}

		b.gateway = NewGateway(token, b.intents, b.wsURL, b.dispatch)
		if resume != nil {
			b.gateway.SetResume(resume.SessionID, resume.LastSeq)
		}

		err := b.gateway.Connect(ctx)
		if err != nil {
			var lf *LoginFailure
			if errors.As(err, &lf) {
				botLog.Info("LOGIN FAILED: %s — check your bot token", err)
				b.terminal = true
				return
			}
			var ge *GatewayError
			if errors.As(err, &ge) {
				botLog.Info("gateway error: %s", err)
			} else {
				botLog.Info("gateway connect failed: %s", err)
			}
		} else {
			connectedAt = time.Now()
			attempts = 0
			runErr := b.gateway.Run(ctx)
			if runErr != nil {
				botLog.Info("gateway run error: %s", runErr)
			}
		}

		if !connectedAt.IsZero() && time.Since(connectedAt) < time.Minute {
			attempts++
		}
		code, reason := b.gateway.CloseInfo()
		switch code {
		case 4001, 4004:
			label := map[int]string{4001: "BANNED", 4004: "INVALID TOKEN"}[code]
			botLog.Info("DISCONNECTED code=%d (%s) reason=%s — not reconnecting", code, label, reason)
			_ = b.gateway.Close()
			b.terminal = true
			return
		case 4010:
			botLog.Info("DISCONNECTED code=4010 (PROTOCOL VIOLATION) reason=%s — will retry", reason)
		}

		if !b.reconnect {
			_ = b.gateway.Close()
			return
		}
		if ctx.Err() != nil {
			_ = b.gateway.Close()
			return
		}

		if b.gateway.InvalidSession() || code == closeInvalidSeq {
			b.SessionID = ""
			resume = nil
		} else if b.SessionID != "" && b.gateway.LastSeq() >= 0 {
			resume = &resumeState{SessionID: b.SessionID, LastSeq: b.gateway.LastSeq()}
		}

		base := 1 << min(attempts, 6)
		if base > 60 {
			base = 60
		}
		jitter := 0.75 + rand.Float64()*0.5
		delay := time.Duration(float64(base)*jitter*float64(time.Second))
		mode := "fresh"
		if resume != nil {
			mode = "resume"
		}
		if code != 0 {
			botLog.Info("disconnected code=%d reason=%s mode=%s — reconnecting in %.1fs (attempt %d)",
				code, reason, mode, delay.Seconds(), attempts+1)
		} else {
			botLog.Info("reconnecting in %.1fs (attempt %d)", delay.Seconds(), attempts+1)
		}
		_ = b.gateway.Close()

		t := time.NewTimer(delay)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return
		}
	}
}

type resumeState struct {
	SessionID string
	LastSeq   int
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (b *Bot) SendMessage(ctx context.Context, channelID, content string) (*Message, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	body, err := h.SendMessage(ctx, channelID, SendMessageBody{Content: content})
	if err != nil {
		return nil, err
	}
	var m Message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	m.client = b.cref
	return &m, nil
}

func (b *Bot) SendEmbed(ctx context.Context, channelID string, embed *Embed) (*Message, error) {
	body := SendMessageBody{Content: ""}
	if embed != nil {
		body.Embed = embed.ToDict()
		if comps := embed.PendingComponents(); comps != nil {
			body.Components = comps
		}
	}
h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	raw, err := h.SendMessage(ctx, channelID, body)
	if err != nil {
		return nil, err
	}
	var m Message
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m.client = b.cref
	return &m, nil
}


const (
	ChannelTypeText  = "text"
	ChannelTypeVoice = "voice"
)

type ChannelOpts struct {
	CategoryID string
	Type       string
}

func (b *Bot) httpClient() (*HTTPClient, error) {
	if b.HTTP == nil {
		return nil, errors.New("banter: bot not running; call HTTP methods from handlers (e.g. OnReady) or after Run starts")
	}
	return b.HTTP, nil
}

type SendFileOpts struct {
	Content string
	ReplyTo string
}

func (b *Bot) DownloadAttachment(ctx context.Context, att *Attachment) (*File, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	if att == nil {
		return nil, errors.New("banter: DownloadAttachment called with nil attachment")
	}
	data, err := h.DownloadAttachment(ctx, att.ID)
	if err != nil {
		return nil, err
	}
	return &File{Data: data, Filename: att.Filename}, nil
}

func (b *Bot) SendFile(ctx context.Context, channelID string, file *File, opts SendFileOpts) (*Message, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, errors.New("banter: SendFile called with nil file")
	}
	raw, err := h.UploadAttachment(ctx, channelID, file)
	if err != nil {
		return nil, err
	}
	var att struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &att); err != nil {
		return nil, err
	}
	body := SendMessageBody{
		Content:       opts.Content,
		ReplyTo:       opts.ReplyTo,
		AttachmentIDs: []string{att.ID},
	}
	out, err := h.SendMessage(ctx, channelID, body)
	if err != nil {
		return nil, err
	}
	var m Message
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, err
	}
	m.client = b.cref
	return &m, nil
}

func (b *Bot) DeleteChannel(ctx context.Context, channelID string) error {
	h, err := b.httpClient()
	if err != nil {
		return err
	}
	_, err = h.DeleteChannel(ctx, channelID)
	return err
}

func (b *Bot) CreateChannel(ctx context.Context, guildID, name string, opts ChannelOpts) (*Channel, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	if opts.Type == "" {
		opts.Type = ChannelTypeText
	}
	raw, err := h.CreateChannel(ctx, guildID, CreateChannelBody{
		Name:       name,
		Type:       opts.Type,
		CategoryID: opts.CategoryID,
	})
	if err != nil {
		return nil, err
	}
	var ch Channel
	if err := json.Unmarshal(raw, &ch); err != nil {
		return nil, err
	}
	ch.client = b.cref
	return &ch, nil
}

func (b *Bot) DeleteCategory(ctx context.Context, categoryID string) error {
	h, err := b.httpClient()
	if err != nil {
		return err
	}
	_, err = h.DeleteCategory(ctx, categoryID)
	return err
}

func (b *Bot) CreateCategory(ctx context.Context, guildID, name string) (*Category, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	raw, err := h.CreateCategory(ctx, guildID, CreateCategoryBody{Name: name})
	if err != nil {
		return nil, err
	}
	var cat Category
	if err := json.Unmarshal(raw, &cat); err != nil {
		return nil, err
	}
	cat.client = b.cref
	return &cat, nil
}

func (b *Bot) ListCategories(ctx context.Context, guildID string) ([]*Category, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	raw, err := h.ListCategories(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var cats []*Category
	if err := json.Unmarshal(raw, &cats); err != nil {
		return nil, err
	}
	for _, cat := range cats {
		cat.client = b.cref
	}
	return cats, nil
}

func (b *Bot) EnsureCategory(ctx context.Context, guildID, name string) (*Category, error) {
	cats, err := b.ListCategories(ctx, guildID)
	if err != nil {
		return nil, err
	}
	for _, cat := range cats {
		if cat.Name == name {
			return cat, nil
		}
	}
	return b.CreateCategory(ctx, guildID, name)
}

func (b *Bot) ListChannels(ctx context.Context, guildID string) ([]*Channel, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	raw, err := h.ListChannels(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var chs []*Channel
	if err := json.Unmarshal(raw, &chs); err != nil {
		return nil, err
	}
	for _, ch := range chs {
		ch.client = b.cref
	}
	return chs, nil
}

func (b *Bot) EnsureChannel(ctx context.Context, guildID, name string, opts ChannelOpts) (*Channel, error) {
	chs, err := b.ListChannels(ctx, guildID)
	if err != nil {
		return nil, err
	}
	for _, ch := range chs {
		if ch.Name == name && ch.CategoryID == opts.CategoryID {
			return ch, nil
		}
	}
	return b.CreateChannel(ctx, guildID, name, opts)
}

func (b *Bot) GetChannel(ctx context.Context, channelID string) (*Channel, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	raw, err := h.GetChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	var ch Channel
	if err := json.Unmarshal(raw, &ch); err != nil {
		return nil, err
	}
	ch.client = b.cref
	return &ch, nil
}

func (b *Bot) GetGuild(ctx context.Context, guildID string) (*Guild, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	raw, err := h.GetGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var g Guild
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, err
	}
	g.client = b.cref
	return &g, nil
}

func (b *Bot) EveryoneRole(ctx context.Context, guildID string) (*Role, error) {
	h, err := b.httpClient()
	if err != nil {
		return nil, err
	}
	raw, err := h.ListRoles(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var roles []*Role
	if err := json.Unmarshal(raw, &roles); err != nil {
		return nil, err
	}
	for _, r := range roles {
		if r.Name == "@everyone" {
			r.client = b.cref
			return r, nil
		}
	}
	return nil, ErrEveryoneNotFound
}