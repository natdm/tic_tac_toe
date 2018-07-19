package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/db"
	"git.tmaws.io/nathan.hyland/tic_tac_toe/game"
	"github.com/gorilla/mux"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type Handler struct {
	game  *game.Game
	store *db.Ref
}

func (h *Handler) GetGame(w http.ResponseWriter, r *http.Request) {
	h.game.WriteTo(w)
}

func (h *Handler) Clear(w http.ResponseWriter, r *http.Request) {
	h.game.Clear()
}

func (h *Handler) Move(w http.ResponseWriter, r *http.Request) {
	var move game.Move
	if err := json.NewDecoder(r.Body).Decode(&move); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	defer r.Body.Close()

	if err := h.game.PlacePiece(move); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) UpdatePlayer(w http.ResponseWriter, r *http.Request) {
	var player game.Player
	if err := json.NewDecoder(r.Body).Decode(&player); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := h.game.UpdatePlayer(player); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var player game.Player
	if err := json.NewDecoder(r.Body).Decode(&player); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(player.ID) == 0 {
		id := uuid.NewV4()
		player.ID = id.String()
	}

	if err := h.game.AddPlayer(player); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"id": "%s"}`, player.ID)))
}

func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	var player game.Player
	if err := json.NewDecoder(r.Body).Decode(&player); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := h.game.RemovePlayer(player.ID); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Restart(w http.ResponseWriter, _ *http.Request) {
	h.game.Clear()
	h.game.NextGame()
	w.WriteHeader(http.StatusOK)
}

// Init initiates the game server with credentials from the user
func (h *Handler) Init(w http.ResponseWriter, r *http.Request) {
	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("error reading body")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	}

	vars := mux.Vars(r)
	projectID, bucket := vars["projectID"], vars["bucket"]

	log.WithFields(log.Fields{
		"projectID": projectID,
		"bucket":    bucket,
	}).Info("initializing...")

	cfg := firebase.Config{
		DatabaseURL:   fmt.Sprintf("https://%s.firebaseio.com", projectID),
		ProjectID:     projectID,
		StorageBucket: bucket,
	}

	err = ioutil.WriteFile("credentials.json", bs, 0644)
	if err != nil {
		log.WithError(err).Error("unable to create credentials file")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	}

	db, err := database(context.Background(), cfg, "credentials.json")
	if err != nil {
		log.WithError(err).Error("unable to create database")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	}

	if err := db.Set(context.Background(), h.game); err != nil {
		log.WithError(err).Error("unable to set db state")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	}

	updateCh := make(chan game.Game)
	h.game.UpdatedCh = updateCh

	go func() {
		// this could be handled much better. Have a context for ending gracefully
		for status := range updateCh {
			if err := db.Set(context.Background(), status); err != nil {
				log.WithError(err).Error("error updating store")
			}
		}
	}()
	log.Info("initialized")
	w.WriteHeader(http.StatusOK)
}

func database(ctx context.Context, cfg firebase.Config, fname string) (*db.Ref, error) {
	opt := option.WithCredentialsFile(fname)
	app, err := firebase.NewApp(ctx, &cfg, opt)
	if err != nil {
		return nil, err
	}

	db, err := app.Database(ctx)
	if err != nil {
		return nil, err
	}

	root := db.NewRef("/")
	return root, nil
}

func Route(g *game.Game, store *db.Ref) (*mux.Router, error) {
	if g == nil {
		return nil, errors.New("need game")
	}
	h := &Handler{g, store}
	r := mux.NewRouter()

	r.HandleFunc("/", h.GetGame).Methods(http.MethodGet)
	r.HandleFunc("/restart", h.Restart).Methods(http.MethodGet)
	r.HandleFunc("/init/project/{projectID}/bucket/{bucket}", h.Init).Methods(http.MethodPost)
	r.HandleFunc("/board/clear", h.Clear).Methods(http.MethodGet)

	p := r.PathPrefix("/player").Subrouter()
	p.HandleFunc("/move", h.Move).Methods(http.MethodPost)
	p.HandleFunc("/update", h.UpdatePlayer).Methods(http.MethodPut)
	p.HandleFunc("/subscribe", h.Subscribe).Methods(http.MethodPost)
	p.HandleFunc("/unsubscribe", h.Unsubscribe).Methods(http.MethodPost)
	return r, nil
}
