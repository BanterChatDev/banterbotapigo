package banter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var httpLog = newLogger("http")

func userAgent() string { return "BanterPy-Go/" + Version + " (+https://banterchat.org)" }

type HTTPClient struct {
	token   string
	baseURL string
	client  *http.Client
}

func NewHTTPClient(token, baseURL string) *HTTPClient {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &HTTPClient{
		token:   token,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (h *HTTPClient) Close() error { return nil }

func (h *HTTPClient) Request(ctx context.Context, method, path string, body any, params map[string]string, extraHeaders map[string]string) ([]byte, error) {
	full := h.baseURL + botBase + path
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Set(k, v)
		}
		full += "?" + q.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(buf)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bot "+h.token)
		req.Header.Set("User-Agent", userAgent())
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}

		httpLog.Debug("HTTP %s %s attempt=%d", method, full, attempt)
		resp, err := h.client.Do(req)
		if err != nil {
			return nil, err
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			return nil, nil
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			httpLog.Debug("HTTP %s %s -> %d", method, full, resp.StatusCode)
			return respBody, nil
		}

		code, message := parseErrorBody(respBody)
		httpLog.Info("HTTP %s %s -> %d code=%d message=%s", method, full, resp.StatusCode, code, message)

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("X-RateLimit-Reset-After"))
			if attempt < maxAttempts-1 {
				if err := sleepOrCancel(ctx, retryAfter); err != nil {
					return nil, err
				}
				if bodyReader != nil {
					bodyReader = bytes.NewReader(mustMarshal(body))
				}
				continue
			}
			httpExc := newHTTPException(resp.StatusCode, code, message, method, path)
			return nil, &RateLimited{HTTPException: *httpExc, RetryAfter: retryAfter}
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, &LoginFailure{BanterError: BanterError{Msg: message}}
		}
		if resp.StatusCode == http.StatusForbidden {
			httpExc := newHTTPException(resp.StatusCode, code, message, method, path)
			return nil, &Forbidden{HTTPException: *httpExc}
		}
		if resp.StatusCode == http.StatusNotFound {
			httpExc := newHTTPException(resp.StatusCode, code, message, method, path)
			return nil, &NotFound{HTTPException: *httpExc}
		}
		if code == 20005 {
			httpExc := newHTTPException(resp.StatusCode, code, message, method, path)
			name := message
			if idx := strings.LastIndex(message, ":"); idx >= 0 {
				name = strings.TrimSpace(message[idx+1:])
			}
			return nil, newDuplicateCommand(name, "server", httpExc)
		}
		return nil, newHTTPException(resp.StatusCode, code, message, method, path)
	}
	return nil, ErrHTTPRetriesExhausted
}

func parseErrorBody(body []byte) (int, string) {
	if len(body) == 0 {
		return 0, "unknown error"
	}
	var p struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return 0, string(body)
	}
	msg := p.Message
	if msg == "" {
		msg = p.Error
	}
	if msg == "" {
		msg = string(body)
	}
	return p.Code, msg
}

func parseRetryAfter(s string) float64 {
	if s == "" {
		return 1.0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v != v || v <= 0 {
		return 1.0
	}
	return v
}

func sleepOrCancel(ctx context.Context, seconds float64) error {
	d := time.Duration(seconds * float64(time.Second))
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (h *HTTPClient) Me(ctx context.Context) ([]byte, error) {
	return h.Request(ctx, "GET", "/users/@me", nil, nil, nil)
}

func (h *HTTPClient) GetUser(ctx context.Context, userID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/users/"+userID, nil, nil, nil)
}

func (h *HTTPClient) GetMember(ctx context.Context, guildID, userID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/guilds/"+guildID+"/members/"+userID, nil, nil, nil)
}

func (h *HTTPClient) ListChannels(ctx context.Context, guildID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/guilds/"+guildID+"/channels", nil, nil, nil)
}

func (h *HTTPClient) GetChannel(ctx context.Context, channelID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/channels/"+channelID, nil, nil, nil)
}

func (h *HTTPClient) ListChannelMembers(ctx context.Context, channelID string, limit, offset int, search string) ([]byte, error) {
	params := map[string]string{
		"limit":  strconv.Itoa(limit),
		"offset": strconv.Itoa(offset),
	}
	if search != "" {
		params["search"] = search
	}
	return h.Request(ctx, "GET", "/channels/"+channelID+"/members", nil, params, nil)
}

func (h *HTTPClient) ListMessages(ctx context.Context, channelID, before string, limit int) ([]byte, error) {
	params := map[string]string{"limit": strconv.Itoa(limit)}
	if before != "" {
		params["before"] = before
	}
	return h.Request(ctx, "GET", "/channels/"+channelID+"/messages", nil, params, nil)
}

type SendMessageBody struct {
	Content       string             `json:"content"`
	Embed         map[string]any     `json:"embed,omitempty"`
	ReplyTo       string             `json:"reply_to,omitempty"`
	AttachmentIDs []string           `json:"attachment_ids,omitempty"`
	Components    []map[string]any   `json:"components,omitempty"`
}

func (h *HTTPClient) SendMessage(ctx context.Context, channelID string, body SendMessageBody) ([]byte, error) {
	return h.Request(ctx, "POST", "/channels/"+channelID+"/messages", body, nil, nil)
}

func (h *HTTPClient) EditMessage(ctx context.Context, messageID, content string) ([]byte, error) {
	return h.Request(ctx, "PATCH", "/messages/"+messageID, map[string]string{"content": content}, nil, nil)
}

func (h *HTTPClient) DeleteMessage(ctx context.Context, messageID string) ([]byte, error) {
	return h.Request(ctx, "DELETE", "/messages/"+messageID, nil, nil, nil)
}

func (h *HTTPClient) TriggerTyping(ctx context.Context, channelID string) ([]byte, error) {
	return h.Request(ctx, "POST", "/channels/"+channelID+"/typing", nil, nil, nil)
}

func (h *HTTPClient) AddReaction(ctx context.Context, channelID, messageID, emoji string) ([]byte, error) {
	return h.Request(ctx, "PUT",
		"/channels/"+channelID+"/messages/"+messageID+"/reactions/"+url.PathEscape(emoji)+"/@me",
		nil, nil, nil)
}

func (h *HTTPClient) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) ([]byte, error) {
	return h.Request(ctx, "DELETE",
		"/channels/"+channelID+"/messages/"+messageID+"/reactions/"+url.PathEscape(emoji)+"/@me",
		nil, nil, nil)
}

func (h *HTTPClient) AddRole(ctx context.Context, guildID, userID, roleID string) ([]byte, error) {
	return h.Request(ctx, "PUT", "/guilds/"+guildID+"/members/"+userID+"/roles/"+roleID, nil, nil, nil)
}

func (h *HTTPClient) RemoveRole(ctx context.Context, guildID, userID, roleID string) ([]byte, error) {
	return h.Request(ctx, "DELETE", "/guilds/"+guildID+"/members/"+userID+"/roles/"+roleID, nil, nil, nil)
}

func (h *HTTPClient) KickMember(ctx context.Context, guildID, userID string) ([]byte, error) {
	return h.Request(ctx, "POST", "/guilds/"+guildID+"/members/"+userID+"/kick", nil, nil, nil)
}

func (h *HTTPClient) BanMember(ctx context.Context, guildID, userID, reason string) ([]byte, error) {
	body := map[string]string{}
	if reason != "" {
		body["reason"] = reason
	}
	return h.Request(ctx, "POST", "/guilds/"+guildID+"/members/"+userID+"/ban", body, nil, nil)
}

func (h *HTTPClient) UnbanMember(ctx context.Context, guildID, userID string) ([]byte, error) {
	return h.Request(ctx, "DELETE", "/guilds/"+guildID+"/members/"+userID+"/ban", nil, nil, nil)
}

func (h *HTTPClient) ListGuildBans(ctx context.Context, guildID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/guilds/"+guildID+"/bans", nil, nil, nil)
}

func (h *HTTPClient) GetGuild(ctx context.Context, guildID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/guilds/"+guildID, nil, nil, nil)
}

type EditGuildPatch struct {
	Name             *string `json:"name,omitempty"`
	Description      *string `json:"description,omitempty"`
	WelcomeChannelID *string `json:"welcome_channel_id,omitempty"`
}

func (h *HTTPClient) EditGuild(ctx context.Context, guildID string, patch EditGuildPatch) ([]byte, error) {
	return h.Request(ctx, "PATCH", "/guilds/"+guildID, patch, nil, nil)
}

func (h *HTTPClient) ListRoles(ctx context.Context, guildID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/guilds/"+guildID+"/roles", nil, nil, nil)
}

func (h *HTTPClient) GetRole(ctx context.Context, roleID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/roles/"+roleID, nil, nil, nil)
}

type CreateChannelBody struct {
	Name                string               `json:"name"`
	Description         string               `json:"description,omitempty"`
	CategoryID          string               `json:"category_id,omitempty"`
	Type                string               `json:"type,omitempty"`
	PermissionOverrides []PermissionOverride `json:"permission_overrides,omitempty"`
}

func (h *HTTPClient) CreateChannel(ctx context.Context, guildID string, body CreateChannelBody) ([]byte, error) {
	return h.Request(ctx, "POST", "/guilds/"+guildID+"/channels", body, nil, nil)
}

type EditChannelPatch struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Position    *int    `json:"position,omitempty"`
	CategoryID  *string `json:"category_id,omitempty"`
}

func (h *HTTPClient) EditChannel(ctx context.Context, channelID string, patch EditChannelPatch) ([]byte, error) {
	return h.Request(ctx, "PUT", "/channels/"+channelID, patch, nil, nil)
}

func (h *HTTPClient) DeleteChannel(ctx context.Context, channelID string) ([]byte, error) {
	return h.Request(ctx, "DELETE", "/channels/"+channelID, nil, nil, nil)
}

type ReorderItem struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
}

func (h *HTTPClient) ReorderChannels(ctx context.Context, guildID string, items []ReorderItem) ([]byte, error) {
	return h.Request(ctx, "PUT", "/guilds/"+guildID+"/channels/reorder", map[string]any{"items": items}, nil, nil)
}

func (h *HTTPClient) ListCategories(ctx context.Context, guildID string) ([]byte, error) {
	return h.Request(ctx, "GET", "/guilds/"+guildID+"/categories", nil, nil, nil)
}

type CreateCategoryBody struct {
	Name                string               `json:"name"`
	PermissionOverrides []PermissionOverride `json:"permission_overrides,omitempty"`
}

func (h *HTTPClient) CreateCategory(ctx context.Context, guildID string, body CreateCategoryBody) ([]byte, error) {
	return h.Request(ctx, "POST", "/guilds/"+guildID+"/categories", body, nil, nil)
}

type EditCategoryPatch struct {
	Name     *string `json:"name,omitempty"`
	Position *int    `json:"position,omitempty"`
}

func (h *HTTPClient) EditCategory(ctx context.Context, categoryID string, patch EditCategoryPatch) ([]byte, error) {
	return h.Request(ctx, "PUT", "/categories/"+categoryID, patch, nil, nil)
}

func (h *HTTPClient) DeleteCategory(ctx context.Context, categoryID string) ([]byte, error) {
	return h.Request(ctx, "DELETE", "/categories/"+categoryID, nil, nil, nil)
}

func (h *HTTPClient) ReorderCategories(ctx context.Context, guildID string, items []ReorderItem) ([]byte, error) {
	return h.Request(ctx, "PUT", "/guilds/"+guildID+"/categories/reorder", map[string]any{"items": items}, nil, nil)
}

func (h *HTTPClient) PurgeChannel(ctx context.Context, channelID string, limit int) ([]byte, error) {
	return h.Request(ctx, "POST", "/channels/"+channelID+"/messages/purge", map[string]int{"limit": limit}, nil, nil)
}

func (h *HTTPClient) SetChannelPermissions(ctx context.Context, channelID, roleID string, allow, deny int64) ([]byte, error) {
	body := map[string]any{"role_id": roleID, "allow": allow, "deny": deny}
	return h.Request(ctx, "PUT", "/channels/"+channelID+"/permissions", body, nil, nil)
}

func (h *HTTPClient) SetCategoryPermissions(ctx context.Context, categoryID, roleID string, allow, deny int64) ([]byte, error) {
	body := map[string]any{"role_id": roleID, "allow": allow, "deny": deny}
	return h.Request(ctx, "PUT", "/categories/"+categoryID+"/permissions", body, nil, nil)
}

type CreateRoleBody struct {
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
	Permissions int64  `json:"permissions,omitempty"`
	Deny        int64  `json:"deny,omitempty"`
	Position    int    `json:"position,omitempty"`
	Mentionable bool   `json:"mentionable,omitempty"`
}

func (h *HTTPClient) CreateRole(ctx context.Context, guildID string, body CreateRoleBody) ([]byte, error) {
	return h.Request(ctx, "POST", "/guilds/"+guildID+"/roles", body, nil, nil)
}

func (h *HTTPClient) EditRole(ctx context.Context, roleID string, patch map[string]any) ([]byte, error) {
	return h.Request(ctx, "PUT", "/roles/"+roleID, patch, nil, nil)
}

func (h *HTTPClient) DeleteRole(ctx context.Context, roleID string) ([]byte, error) {
	return h.Request(ctx, "DELETE", "/roles/"+roleID, nil, nil, nil)
}

func (h *HTTPClient) RegisterCommands(ctx context.Context, commands []map[string]any) ([]byte, error) {
	body := map[string]any{"commands": commands}
	return h.Request(ctx, "PUT", "/applications/@me/commands", body, nil, nil)
}

func (h *HTTPClient) ListCommands(ctx context.Context) ([]byte, error) {
	return h.Request(ctx, "GET", "/applications/@me/commands", nil, nil, nil)
}

func (h *HTTPClient) RespondInteraction(ctx context.Context, interactionID, token string, body map[string]any) error {
	headers := map[string]string{"X-Interaction-Token": token}
	_, err := h.Request(ctx, "POST", "/interactions/"+interactionID+"/respond", body, nil, headers)
	return err
}

func (h *HTTPClient) DownloadAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	full := h.baseURL + "/api/v1/attachments/" + attachmentID
	req, err := http.NewRequestWithContext(ctx, "GET", full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+h.token)
	req.Header.Set("User-Agent", userAgent())
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, newHTTPException(resp.StatusCode, 0,
			fmt.Sprintf("download_attachment %s -> %d", attachmentID, resp.StatusCode),
			"GET", "/attachments/"+attachmentID)
	}
	return body, nil
}

func (h *HTTPClient) UploadAttachment(ctx context.Context, channelID string, f *File) ([]byte, error) {
	path := "/attachments"
	full := h.baseURL + botBase + path
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("channel_id", channelID)
		fw, err := w.CreateFormFile("file", f.Filename)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(f.Data); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", full, &buf)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bot "+h.token)
		req.Header.Set("User-Agent", userAgent())
		req.Header.Set("Content-Type", w.FormDataContentType())
		resp, err := h.client.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}
		code, message := parseErrorBody(body)
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("X-RateLimit-Reset-After"))
			if attempt < maxAttempts-1 {
				if err := sleepOrCancel(ctx, retryAfter); err != nil {
					return nil, err
				}
				continue
			}
			httpExc := newHTTPException(resp.StatusCode, code, message, "POST", path)
			return nil, &RateLimited{HTTPException: *httpExc, RetryAfter: retryAfter}
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, &LoginFailure{BanterError: BanterError{Msg: message}}
		}
		if resp.StatusCode == http.StatusForbidden {
			httpExc := newHTTPException(resp.StatusCode, code, message, "POST", path)
			return nil, &Forbidden{HTTPException: *httpExc}
		}
		if resp.StatusCode == http.StatusNotFound {
			httpExc := newHTTPException(resp.StatusCode, code, message, "POST", path)
			return nil, &NotFound{HTTPException: *httpExc}
		}
		return nil, newHTTPException(resp.StatusCode, code, message, "POST", path)
	}
	return nil, ErrUploadRetriesExhausted
}