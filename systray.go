//go:build !cli

package main

import (
	_ "embed"
	"fmt"
	"sync"

	"app/backend"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed build/appicon.png
var appIcon []byte

// SystemTrayController manages the system tray icon, context menu, and background window lifecycle.
type SystemTrayController struct {
	app           *application.App
	window        *application.WebviewWindow
	configMgr     *backend.ConfigManager
	tray          *application.SystemTray
	statusItem    *application.MenuItem
	syncNowItem   *application.MenuItem
	toggleItem    *application.MenuItem
	showItem      *application.MenuItem
	quitItem      *application.MenuItem
	mu            sync.Mutex
	currentStatus backend.AutoSyncStatus
}

// setupSystemTray initializes the system tray, registers window hooks, and builds the context menu.
func setupSystemTray(app *application.App, window *application.WebviewWindow, configMgr *backend.ConfigManager) *SystemTrayController {
	c := &SystemTrayController{
		app:       app,
		window:    window,
		configMgr: configMgr,
	}

	c.initWindowLifecycle()
	c.initTray()
	c.initEventListener()

	return c
}

func (c *SystemTrayController) showMainWindow() {
	if c.window == nil {
		return
	}
	c.window.Show()
	c.window.Restore()
	c.window.Focus()
}

func (c *SystemTrayController) initWindowLifecycle() {
	if c.window == nil {
		return
	}

	// Cancel window close and hide window instead of exiting
	c.window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		c.window.Hide()
	})

	// When user clicks minimize button, hide to system tray
	c.window.OnWindowEvent(events.Common.WindowMinimise, func(event *application.WindowEvent) {
		c.window.Hide()
	})
}

func (c *SystemTrayController) initTray() {
	c.tray = c.app.SystemTray.New()
	if len(appIcon) > 0 {
		c.tray.SetIcon(appIcon)
	}
	c.tray.SetTooltip(title)

	menu := c.app.NewMenu()

	// 1. Auto-Sync Status display item
	c.statusItem = menu.Add("Trạng thái: Đang theo dõi")
	c.statusItem.SetEnabled(false)

	menu.AddSeparator()

	// 2. Sync Now
	c.syncNowItem = menu.Add("Đồng bộ ngay bây giờ")
	c.syncNowItem.OnClick(func(_ *application.Context) {
		go func() {
			if c.configMgr != nil {
				_ = c.configMgr.TriggerSyncNow()
			}
		}()
	})

	// 3. Pause / Resume Sync
	c.toggleItem = menu.Add("Tạm dừng đồng bộ")
	c.toggleItem.OnClick(func(_ *application.Context) {
		status := c.configMgr.GetAutoSyncStatus()
		c.configMgr.SetAutoSyncEnabled(!status.Enabled)
	})

	menu.AddSeparator()

	// 4. Open main UI
	c.showItem = menu.Add("Mở giao diện chính")
	c.showItem.OnClick(func(_ *application.Context) {
		c.showMainWindow()
	})

	// 5. Quit
	c.quitItem = menu.Add("Thoát")
	c.quitItem.OnClick(func(_ *application.Context) {
		c.app.Quit()
	})

	c.tray.SetMenu(menu)

	// Left-click and double-click restore the window
	c.tray.OnClick(func() {
		c.showMainWindow()
	})
	c.tray.OnDoubleClick(func() {
		c.showMainWindow()
	})

	// Right-click refreshes status before opening menu
	c.tray.OnRightClick(func() {
		c.refreshStatus()
		c.tray.OpenMenu()
	})
}

func (c *SystemTrayController) refreshStatus() {
	if c.configMgr == nil {
		return
	}
	status := c.configMgr.GetAutoSyncStatus()
	c.updateStatus(status)
}

func (c *SystemTrayController) updateStatus(status backend.AutoSyncStatus) {
	c.mu.Lock()
	c.currentStatus = status
	c.mu.Unlock()

	var statusLabel string
	var toggleLabel string
	var tooltipText string

	if !status.Enabled {
		statusLabel = "Tự động đồng bộ: Đã tạm dừng"
		toggleLabel = "Tiếp tục đồng bộ"
		tooltipText = title + " - Tự động đồng bộ tắt"
	} else {
		toggleLabel = "Tạm dừng đồng bộ"
		if status.IsSyncing {
			if status.QueueCount > 0 {
				statusLabel = fmt.Sprintf("Đang tải lên (%d file)", status.QueueCount)
				tooltipText = fmt.Sprintf("%s - Đang tải lên (%d file)", title, status.QueueCount)
			} else if status.CurrentFile != "" {
				statusLabel = fmt.Sprintf("Đang tải lên: %s", status.CurrentFile)
				tooltipText = fmt.Sprintf("%s - Đang tải lên: %s", title, status.CurrentFile)
			} else {
				statusLabel = "Đang tải lên..."
				tooltipText = title + " - Đang tải lên"
			}
		} else {
			if status.FolderCount > 0 {
				statusLabel = fmt.Sprintf("Đang theo dõi (%d thư mục)", status.FolderCount)
			} else {
				statusLabel = "Đang theo dõi"
			}
			tooltipText = title + " - " + statusLabel
		}
	}

	if c.statusItem != nil {
		c.statusItem.SetLabel(statusLabel)
	}
	if c.toggleItem != nil {
		c.toggleItem.SetLabel(toggleLabel)
	}
	if c.tray != nil {
		c.tray.SetTooltip(tooltipText)
	}
}

func (c *SystemTrayController) initEventListener() {
	if c.app == nil {
		return
	}

	c.app.Event.On("autosync:status", func(e *application.CustomEvent) {
		if status, ok := e.Data.(backend.AutoSyncStatus); ok {
			c.updateStatus(status)
		} else if statusPtr, ok := e.Data.(*backend.AutoSyncStatus); ok && statusPtr != nil {
			c.updateStatus(*statusPtr)
		}
	})

	// Initial status update
	c.refreshStatus()
}
