//go:build !cli

package backend

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var (
	wailsAppInstance *application.App
	inAppAuthMu      sync.Mutex
	activeLoginWin   *application.WebviewWindow
	activeCancelFn   context.CancelFunc

	inAppPhotosMu   sync.Mutex
	activePhotosWin *application.WebviewWindow
)

// SetApp saves the Wails application reference for window management.
func (g *ConfigManager) SetApp(app *application.App) {
	configMu.Lock()
	defer configMu.Unlock()
	wailsAppInstance = app
}

// StartInAppGoogleLogin opens a dedicated in-app WebView window for Google EmbeddedSetup,
// polls for the oauth_token cookie, automatically closes the window upon successful login,
// and saves the new Google Photos account.
func (g *ConfigManager) StartInAppGoogleLogin() (email string, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[InAppAuth] Panic recovered in StartInAppGoogleLogin: %v\n", r)
			err = errors.New("sign-in encountered an internal error")
		}
	}()

	inAppAuthMu.Lock()
	if activeLoginWin != nil {
		inAppAuthMu.Unlock()
		return "", errors.New("a Google login window is already open")
	}

	app := wailsAppInstance
	if app == nil {
		inAppAuthMu.Unlock()
		return "", errors.New("application instance is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	activeCancelFn = cancel

	loginWin := application.InvokeSyncWithResult(func() *application.WebviewWindow {
		w := app.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:               "Google Sign-In - Google Photos",
			Width:               500,
			Height:              680,
			URL:                 "https://accounts.google.com/EmbeddedSetup",
			DisableResize:       false,
			MaximiseButtonState: application.ButtonDisabled,
			BackgroundType:      application.BackgroundTypeSolid,
		})
		w.Center()
		w.Show()
		w.Focus()
		return w
	})

	activeLoginWin = loginWin
	inAppAuthMu.Unlock()

	defer func() {
		inAppAuthMu.Lock()
		activeCancelFn = nil
		activeLoginWin = nil
		inAppAuthMu.Unlock()
	}()

	windowClosed := make(chan struct{})
	var closeOnce sync.Once
	var isSuccess bool

	loginWin.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		closeOnce.Do(func() {
			close(windowClosed)
		})
		if !isSuccess {
			cancel()
		}
	})

	// Wait 1s for WebView2 and page DOM to initialize before polling cookies
	select {
	case <-ctx.Done():
		return "", errors.New("sign-in window closed")
	case <-time.After(1000 * time.Millisecond):
	}

	ticker := time.NewTicker(600 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			select {
			case <-windowClosed:
				// Final check in case token was saved right as window closed
				if finalToken, err := extractOAuthTokenFromSQLite(); err == nil && len(finalToken) >= 16 {
					log.Printf("[InAppAuth] Captured oauth_token from disk after window close, finishing...\n")
					return g.AddGoogleAccount(finalToken)
				}
				return "", errors.New("login window was closed")
			default:
				return "", errors.New("login timed out")
			}
		case <-ticker.C:
			token, err := extractOAuthTokenFromWindow(loginWin)
			if err != nil || token == "" {
				continue
			}
			if len(token) >= 16 {
				log.Printf("[InAppAuth] Successfully captured oauth_token (length=%d), finishing account connect...\n", len(token))
				isSuccess = true
				loginWin.Close()
				return g.AddGoogleAccount(token)
			}
		}
	}
}

// CancelInAppGoogleLogin closes the active in-app Google login window if one is open.
func (g *ConfigManager) CancelInAppGoogleLogin() error {
	inAppAuthMu.Lock()
	defer inAppAuthMu.Unlock()

	if activeCancelFn != nil {
		activeCancelFn()
		activeCancelFn = nil
	}
	if activeLoginWin != nil {
		activeLoginWin.Close()
		activeLoginWin = nil
	}
	return nil
}

// OpenInAppGooglePhotos opens a Google Photos URL inside a dedicated in-app WebView window.
func (g *ConfigManager) OpenInAppGooglePhotos(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		rawURL = "https://photos.google.com/"
	}

	app := wailsAppInstance
	if app == nil {
		return errors.New("application instance is not initialized")
	}

	inAppPhotosMu.Lock()
	defer inAppPhotosMu.Unlock()

	if activePhotosWin != nil {
		application.InvokeSync(func() {
			activePhotosWin.SetURL(rawURL)
			activePhotosWin.Show()
			activePhotosWin.Restore()
			activePhotosWin.Focus()
		})
		return nil
	}

	photosWin := application.InvokeSyncWithResult(func() *application.WebviewWindow {
		w := app.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:               "Google Photos",
			Width:               1060,
			Height:              720,
			URL:                 rawURL,
			DisableResize:       false,
			MaximiseButtonState: application.ButtonEnabled,
			BackgroundType:      application.BackgroundTypeSolid,
		})
		w.Center()
		w.Show()
		w.Focus()
		return w
	})

	activePhotosWin = photosWin
	photosWin.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		inAppPhotosMu.Lock()
		if activePhotosWin == photosWin {
			activePhotosWin = nil
		}
		inAppPhotosMu.Unlock()
	})

	return nil
}
