package banter

import (
	"errors"
	"fmt"
)

const (
	MsgDupSlashCommand    = "duplicate slash command %q — fix your command loader and restart"
	MsgDupPrefixCommand   = "duplicate prefix command %q — fix your command loader and restart"
	MsgDupButtonHandler   = "duplicate button handler %q — fix your loader and restart"
	MsgEmptyButtonID      = "OnButton: custom_id must be non-empty"
	MsgCommandSyncDup     = "command sync rejected by server: duplicate name %q in registration payload. The server did NOT modify your previously-registered commands. Check your command loader for a duplicate entry, or rename one of the commands."
	MsgSlashHandlerFailed = "Command failed."
)

var (
	ErrInteractionDetached    = errors.New("interaction has no attached client")
	ErrInteractionHTTPMissing = errors.New("interaction has no attached HTTP client")
	ErrInteractionResponded   = errors.New("interaction already responded to")
	ErrRespondEmpty           = errors.New("respond requires non-empty content or an embed")
	ErrFollowupEmpty          = errors.New("followup requires non-empty content or an embed")
	ErrUpdateNotButton        = errors.New("update is only valid for button interactions — use Respond or Followup for slash commands")
	ErrContextDetached        = errors.New("context detached from bot")
	ErrFileNeedsFilename      = errors.New("file constructors require a filename")
	ErrHTTPRetriesExhausted   = errors.New("http: exhausted retries without returning")
	ErrUploadRetriesExhausted = errors.New("upload: exhausted retries")
	ErrEveryoneNotFound       = errors.New("@everyone role not found in guild")
)

type BanterError struct {
	Msg string
}

func (e *BanterError) Error() string { return e.Msg }

type HTTPException struct {
	BanterError
	Status  int
	Code    int
	Message string
	Method  string
	Path    string
}

func newHTTPException(status, code int, message, method, path string) *HTTPException {
	var msg string
	if method != "" && path != "" {
		msg = fmt.Sprintf("%s %s -> %d %d: %s", method, path, status, code, message)
	} else {
		msg = fmt.Sprintf("%d %d: %s", status, code, message)
	}
	return &HTTPException{
		BanterError: BanterError{Msg: msg},
		Status:      status,
		Code:        code,
		Message:     message,
		Method:      method,
		Path:        path,
	}
}

type Forbidden struct{ HTTPException }
type NotFound struct{ HTTPException }

type RateLimited struct {
	HTTPException
	RetryAfter float64
}

type DuplicateCommand struct {
	BanterError
	Name    string
	Source  string
	HTTPExc *HTTPException
}

func newDuplicateCommand(name, source string, httpExc *HTTPException) *DuplicateCommand {
	var msg string
	if source == "server" {
		msg = fmt.Sprintf("server rejected duplicate command name: %q", name)
	} else {
		msg = fmt.Sprintf("duplicate slash command name: %q is already registered", name)
	}
	return &DuplicateCommand{
		BanterError: BanterError{Msg: msg},
		Name:        name,
		Source:      source,
		HTTPExc:     httpExc,
	}
}

type GatewayError struct{ BanterError }
type LoginFailure struct{ BanterError }

type MissingPermissions struct {
	BanterError
	Missing []string
}

func newMissingPermissions(missing []string) *MissingPermissions {
	names := "unknown"
	if len(missing) > 0 {
		names = ""
		for i, n := range missing {
			if i > 0 {
				names += ", "
			}
			names += n
		}
	}
	return &MissingPermissions{
		BanterError: BanterError{Msg: "Missing permissions: " + names},
		Missing:     missing,
	}
}