//go:build windows

package main

import (
	"context"
	_ "embed"
	"sync"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var trayOnce sync.Once

//go:embed build/windows/icon.ico
var trayIcon []byte

func startTray(ctx context.Context) {
	if ctx == nil {
		return
	}

	trayOnce.Do(func() {
		go systray.Run(func() {
			if len(trayIcon) > 0 {
				systray.SetIcon(trayIcon)
			}
			systray.SetTitle("Zapret UI")
			systray.SetTooltip("zapret-ui")

			mOpen := systray.AddMenuItem("Open", "Show the main window")
			mHide := systray.AddMenuItem("Hide", "Hide the main window")
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("Exit", "Exit the application")

			go func() {
				for {
					select {
					case <-mOpen.ClickedCh:
						runtime.WindowShow(ctx)
						runtime.WindowUnminimise(ctx)
						// Wails v2 runtime doesn't expose an explicit focus API; show + unminimise is sufficient.
					case <-mHide.ClickedCh:
						runtime.WindowHide(ctx)
					case <-mQuit.ClickedCh:
						runtime.Quit(ctx)
						systray.Quit()
						return
					}
				}
			}()
		}, func() {})
	})
}
