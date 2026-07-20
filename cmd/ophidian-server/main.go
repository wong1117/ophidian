package main

import (
	"log"
	"github.com/ophidian/ophidian/internal/infrastructure/web"
	"github.com/ophidian/ophidian/internal/infrastructure/web/handlers"
)

func main() {
	server := web.NewServer()

	mh := handlers.NewMissionHandler()
	rh := handlers.NewReconHandler()
	eh := handlers.NewExploitHandler()
	ah := handlers.NewAIHandler()
	rph := handlers.NewReportHandler()

	web.RegisterRoutes(server.Echo, mh, rh, eh, ah, rph)

	if err := server.Start(":8443"); err != nil {
		log.Fatal(err)
	}
}
