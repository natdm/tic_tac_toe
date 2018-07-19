package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"git.tmaws.io/nathan.hyland/tic_tac_toe/game"
	"github.com/rs/cors"
	logger "github.com/sirupsen/logrus"
)

func main() {
	log := logger.New()
	g := game.New(log.WithField("package", "game_engine"), 5*time.Second, nil)

	r, err := Route(g, nil)
	if err != nil {
		log.Fatalln(err)
	}

	handler := cors.Default().Handler(r)
	port := ":8080"
	if p, ok := os.LookupEnv("PORT"); ok {
		port = fmt.Sprintf(":%s", p)
	}
	log.Fatalln(http.ListenAndServe(port, handler))
}
