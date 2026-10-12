// Seismic: a native macOS Elasticsearch client in pure Go (mygo native UI).
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	appui "seismic/internal/ui"
)

func main() {
	a := appui.NewApp(ui.NewRouter("/overview"))
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "Seismic",
			Width:     1280,
			Height:    800,
			MinWidth:  960,
			MinHeight: 600,
			StateKey:  "main",
			Content:   ui.View(a.View),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
