// Seismic: a native macOS Elasticsearch client in pure Go (mygo native UI).
package main

import (
	"context"
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"seismic/internal/conn"
	"seismic/internal/db"
	appui "seismic/internal/ui"
)

func main() {
	ctx := context.Background()
	a := appui.NewApp(ui.NewRouter("/overview"))

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "Seismic",
			Width:     1280,
			Height:    800,
			MinWidth:  960,
			MinHeight: 600,
			StateKey:  "main",
			// 侧边栏材质：透明页面上透出系统毛玻璃，标题栏随之融合。
			Vibrancy:  mygo.VibrancySidebar,
			Content:   ui.View(a.View),
		})
		a.Attach(win, ctx)

		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			log.Printf("user data dir: %v", err)
			return
		}
		secrets := conn.NewSecretStore(dir)
		database, err := db.Open(ctx, filepath.Join(dir, "seismic.db"))
		if err != nil {
			log.Printf("open database: %v", err)
		}
		conns, err := conn.NewStore(dir, secrets)
		if err != nil {
			log.Printf("connections: %v", err)
			return
		}
		a.AttachDeps(appui.Deps{Conns: conns, DB: database, HomeDir: dir})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
