package core

import (
	"fmt"
	"strings"

	"dinkisstyle-chat/internal/googleintegration"
	"github.com/pkg/browser"
)

func (a *App) googleManager() *googleintegration.Manager {
	a.googleOnce.Do(func() { a.google = googleintegration.New(joinAppDataPath("google-integration")) })
	return a.google
}

// These settings are accessible only through the native server management UI.
// OAuth tokens and client secrets never appear in web account/config responses.
func (a *App) GetUserGoogleStatus(id string) (googleintegration.Status, error) {
	if _, err := a.authMgr.GetUserDetail(id); err != nil {
		return googleintegration.Status{}, err
	}
	return a.googleManager().Status(id)
}
func (a *App) SetUserGoogleSettings(id string, settings googleintegration.Settings) error {
	if _, err := a.authMgr.GetUserDetail(id); err != nil {
		return err
	}
	settings.ClientID = strings.TrimSpace(settings.ClientID)
	settings.ClientSecret = strings.TrimSpace(settings.ClientSecret)
	if settings.ClientID != "" && !strings.HasSuffix(settings.ClientID, ".apps.googleusercontent.com") {
		return fmt.Errorf("enter a Google Desktop OAuth client ID ending in .apps.googleusercontent.com")
	}
	return a.googleManager().Configure(id, settings)
}
func (a *App) ConnectUserGoogle(id string) error {
	if _, err := a.authMgr.GetUserDetail(id); err != nil {
		return err
	}
	link, err := a.googleManager().Begin(id)
	if err != nil {
		return err
	}
	return browser.OpenURL(link)
}
func (a *App) DisconnectUserGoogle(id string) error {
	if _, err := a.authMgr.GetUserDetail(id); err != nil {
		return err
	}
	return a.googleManager().Disconnect(id, false)
}
