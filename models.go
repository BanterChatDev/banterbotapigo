package banter

import "encoding/json"

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarID    string `json:"avatar_id"`
	Bot         bool   `json:"is_bot"`
}

func (u *User) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		AvatarID    string `json:"avatar_id"`
		AvatarAlt   string `json:"avatar"`
		IsBot       bool   `json:"is_bot"`
		IsBotAlt    bool   `json:"bot"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	u.ID = raw.ID
	u.Username = raw.Username
	u.DisplayName = raw.DisplayName
	u.AvatarID = raw.AvatarID
	if u.AvatarID == "" {
		u.AvatarID = raw.AvatarAlt
	}
	u.Bot = raw.IsBot || raw.IsBotAlt
	return nil
}

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Position    int    `json:"position"`
	Permissions int64  `json:"permissions"`

	client *clientRef `json:"-"`
}

type Member struct {
	UserID   string   `json:"user_id"`
	GuildID  string   `json:"guild_id"`
	Nickname string   `json:"nickname"`
	RoleIDs  []string `json:"role_ids"`
	JoinedAt string   `json:"joined_at"`

	client *clientRef `json:"-"`
}

type Channel struct {
	ID          string `json:"id"`
	GuildID     string `json:"guild_id"`
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Position    int    `json:"position"`

	client *clientRef `json:"-"`
}

type Category struct {
	ID       string `json:"id"`
	GuildID  string `json:"guild_id"`
	Name     string `json:"name"`
	Position int    `json:"position"`

	client *clientRef `json:"-"`
}

type Guild struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	OwnerID           string `json:"owner_id"`
	WelcomeChannelID  string `json:"welcome_channel_id"`

	client *clientRef `json:"-"`
}

type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	URL      string `json:"url"`
}

type Message struct {
	ID            string       `json:"id"`
	ChannelID     string       `json:"channel_id"`
	GuildID       string       `json:"guild_id"`
	UserID        string       `json:"user_id"`
	Content       string       `json:"content"`
	CreatedAt     string       `json:"created_at"`
	EditedAt      string       `json:"edited_at"`
	ReplyTo       string       `json:"reply_to"`
	Attachments   []Attachment `json:"attachments"`
	IsBot         bool         `json:"is_bot"`

	client *clientRef `json:"-"`
}

type clientRef struct {
	http httpDoer
}

type httpDoer interface{}