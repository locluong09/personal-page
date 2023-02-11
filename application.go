package main

import (
	"go-site/src"
	"log"
)

func main() {
	app, err := src.NewApplication(src.NewConfig())
	if err != nil {
		log.Fatalf("Oops there is an error: %v", err)
	}

	server := src.NewServer(app.Config, &app.Handler)

	log.Printf("listening on port %s\n", app.Config.Port)
	log.Fatal(server.ListenAndServe())
}
