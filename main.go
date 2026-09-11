//go:build !cli

package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"strings"

	"app/backend"
	"app/internal/cli"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

var title = "gotohp v" + getAppVersion()

func main() {
	// Run the CLI when the arguments name one of its commands; otherwise start the GUI.
	if cli.IsCLIInvocation(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:], cli.Info{
			ExecutableName: "gotohp",
			HasGUI:         true,
			Version:        getAppVersion(),
		}))
	}

	runGUI()
}

func runGUI() {
	normalizeFrontendDevServerURL()
	backend.SyncAutostart()

	startHidden := hasHiddenFlag(os.Args[1:])
	configManager := &backend.ConfigManager{}
	var window *application.WebviewWindow

	wailsApp := application.New(application.Options{
		Name:        "com.nhanhq.gotohp",
		Description: "Google Photos unofficial client",
		Services: []application.Service{
			application.NewService(configManager),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.nhanhq.gotohp",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if hasHiddenFlag(data.Args) {
					return
				}
				if window != nil {
					window.Show()
					window.Restore()
					window.Focus()
				}
			},
		},
	})
	configManager.SetApp(wailsApp)

	window = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:               title,
		Frameless:           false,
		Width:               400,
		Height:              600,
		EnableFileDrop:      true,
		DisableResize:       true,
		MaximiseButtonState: application.ButtonDisabled,
		BackgroundType:      application.BackgroundTypeTranslucent,
		Hidden:              startHidden,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 0,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	uploadManager := backend.NewUploadManager(backend.NewWailsReporter(wailsApp), wailsApp.Logger)
	uploadManager.SetHistoryStore(backend.GetHistoryStore())

	// Listen for upload cancel event
	wailsApp.Event.On("uploadCancel", func(e *application.CustomEvent) {
		uploadManager.Cancel()
	})

	// Listen for upload pause event
	wailsApp.Event.On("uploadPause", func(e *application.CustomEvent) {
		uploadManager.Pause()
		wailsApp.Event.Emit("uploadPaused", nil)
	})

	// Listen for upload resume event
	wailsApp.Event.On("uploadResume", func(e *application.CustomEvent) {
		uploadManager.Resume()
		wailsApp.Event.Emit("uploadResumed", nil)
	})

	// Listen for bandwidth limit event (bytes/sec)
	wailsApp.Event.On("uploadSetBandwidthLimit", func(e *application.CustomEvent) {
		if limitBytes, ok := e.Data.(int64); ok {
			uploadManager.SetBandwidthLimit(limitBytes)
		} else if limitFloat, ok := e.Data.(float64); ok {
			uploadManager.SetBandwidthLimit(int64(limitFloat))
		}
	})

	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		paths := event.Context().DroppedFiles()
		dropTarget := event.Context().DropTargetDetails()

		var dropZone string
		if dropTarget != nil {
			dropZone = dropTarget.Attributes["data-drop-zone"]
			wailsApp.Logger.Info("Drop target detected",
				"dropZone", dropZone,
				"elementID", dropTarget.ElementID)
		}

		// Emit event to frontend with drop details
		wailsApp.Event.Emit("files-dropped", backend.FilesDroppedEvent{
			Files:    paths,
			DropZone: dropZone,
		})
	})

	// Listen for upload request from frontend (after drop zone is determined)
	wailsApp.Event.On("startUpload", func(e *application.CustomEvent) {
		if data, ok := e.Data.(backend.StartUploadEvent); ok {
			wailsApp.Logger.Info("Starting upload", "fileCount", len(data.Files))
			uploadManager.Upload(data.Files, configManager.SessionUploadOptions())
		} else {
			wailsApp.Logger.Error("startUpload: unexpected data type", "type", fmt.Sprintf("%T", e.Data))
		}
	})

	autoSyncNotifier := backend.NewWailsAutoSyncNotifier(wailsApp)
	autoSyncManager, err := backend.NewAutoSyncManager(configManager, autoSyncNotifier, wailsApp.Logger)
	if err == nil {
		backend.SetActiveAutoSyncManager(autoSyncManager)
		autoSyncManager.Start()
		defer autoSyncManager.Stop()
	} else {
		wailsApp.Logger.Error("failed to initialize AutoSyncManager", "error", err)
	}

	setupSystemTray(wailsApp, window, configManager)

	err = wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func normalizeFrontendDevServerURL() {
	const envName = "FRONTEND_DEVSERVER_URL"

	value := os.Getenv(envName)
	value = strings.Replace(value, "http://localhost:", "http://127.0.0.1:", 1)
	value = strings.Replace(value, "https://localhost:", "https://127.0.0.1:", 1)
	if value != os.Getenv(envName) {
		_ = os.Setenv(envName, value)
	}
}

func hasHiddenFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--hidden" || arg == "-hidden" || arg == "--systray" || arg == "-systray" || arg == "--minimized" {
			return true
		}
	}
	return false
}
