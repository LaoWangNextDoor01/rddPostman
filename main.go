package main

import (
	"log"

	"rddpostman/internal/app"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	a := app.New()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "rddPostman",
			Width:     1280,
			Height:    800,
			MinWidth:  960,
			MinHeight: 600,
			// Opens where the user left it last time.
			StateKey: "main",
			// The view draws the title bar under the window controls.
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(a.View),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
