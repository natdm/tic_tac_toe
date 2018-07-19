package game

import (
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
)

// Game level errors
var (
	ErrPlayerNotFound = errors.New("player not found")
	ErrGameInProgress = errors.New("game in progress")
	ErrInvalidMove    = errors.New("invalid move")
)

// Piece is either X, Y, or blank
type Piece int

const (
	blank  Piece = 0
	xPiece Piece = -1
	oPiece Piece = 1

	xWins = -3
	oWins = 3
)

// Board represents a game board that holds piece positions
// x 	 = -1
// o 	 = 1
// empty = 0
type Board [3][3]Piece

// Player represents a user either playing or waiting in queue.
// since name is optional, it can be nil
type Player struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

// Status is a game status
type Status string

// ..
func (s Status) String() string {
	switch s {
	case InsufficientPlayers:
		return "InsufficientPlayers"
	case NoBoard:
		return "NoBoard"
	case XWins:
		return "XWins"
	case OWins:
		return "OWins"
	case Cats:
		return "Cats"
	case InProgress:
		return "InProgress"
	}
	return "No status?"
}

// Move is a move input from a player
type Move struct {
	YAxis    int    `json:"y_axis"`
	XAxis    int    `json:"x_axis"`
	PlayerID string `json:"player_id"`
}

// Status are all possible game statuses
const (
	InsufficientPlayers Status = "InsufficientPlayers"
	NoBoard             Status = "NoBoard"
	XWins               Status = "XWins"
	OWins               Status = "OWins"
	Cats                Status = "Cats"
	InProgress          Status = "InProgress"
)

func (s Status) ptr() *string {
	x := string(s)
	return &x
}

// Game represents the entire Game state, including current X and Y,
// the board, and the queue.
type Game struct {
	Board  *Board   `json:"board"`
	Queue  []Player `json:"queue"`
	X      *Player  `json:"player_x"`
	O      *Player  `json:"player_o"`
	Move   string   `json:"move,omitempty"`
	Status Status   `json:"status"`
	log    *logrus.Entry

	UpdatedCh chan<- Game `json:"-"`
	timeout   time.Duration

	resetTimeoutCh, stopTimeoutCh chan struct{}
}

// New returns a new game instance.
// X is -1
// Y is 1
func New(logger *logrus.Entry, timeout time.Duration, ch chan<- Game) *Game {
	rand.Seed(time.Now().UnixNano())

	board := Board([3][3]Piece{
		{blank, blank, blank},
		{blank, blank, blank},
		{blank, blank, blank}})

	if logger == nil {
		logger = logrus.New().WithField("package", "game")
	}

	g := &Game{
		Board:     &board,
		Queue:     []Player{},
		Status:    InsufficientPlayers,
		UpdatedCh: ch,
		log:       logger,
		timeout:   timeout,

		resetTimeoutCh: make(chan struct{}),
		stopTimeoutCh:  make(chan struct{}),
	}
	return g
}

func (g *Game) String() string {
	if g == nil {
		return ""
	}

	bs, err := json.Marshal(g)
	if err != nil {
		return ""
	}
	return string(bs)
}

func (g *Game) update() {
	if g.UpdatedCh != nil {
		go func() { g.UpdatedCh <- *g }()
	}
}

// WriteTo ...
func (g *Game) WriteTo(w io.Writer) error {
	return json.NewEncoder(w).Encode(g)
}

func (g *Game) Clear() {
	g = New(g.log, g.timeout, g.UpdatedCh)
}

func (g *Game) clearBoard() {
	b := Board([3][3]Piece{
		{blank, blank, blank},
		{blank, blank, blank},
		{blank, blank, blank}})

	g.Board = &b
}

// advanceQueue takes a player to add to the back of the queue and
// returns the player at the top of the queue to advance to the next spot
func (g *Game) advanceQueue(loser *Player) (next *Player) {
	if loser != nil {
		g.Queue = append(g.Queue, *loser)
	}
	if len(g.Queue) > 0 {
		next = &g.Queue[0]
		g.Queue = g.Queue[1:]
	}
	return
}

// NextGame advances to the next game, adjusting the queue and the board.
func (g *Game) NextGame() error {
	switch g.Status {
	case XWins:
		g.Move = "O"
		g.clearBoard()
		g.O = g.advanceQueue(g.O)

	case OWins:
		g.Move = "X"
		g.clearBoard()
		g.X = g.advanceQueue(g.X)

	case Cats:
		// randomly pick between X and Y to lose. Send the loser to the
		// bottom of the queue but make the next in queue the first to
		// move
		xWins := rand.Float32() < 0.5

		if xWins {
			g.Move = "O"
			g.O = g.advanceQueue(g.O)

		} else {
			g.Move = "X"
			g.X = g.advanceQueue(g.X)
		}
		g.clearBoard()

	case InProgress:
		// someone probably quit, expect a nil X or Y
		if g.X == nil {
			g.X = g.advanceQueue(nil)
		}
		if g.O == nil {
			g.O = g.advanceQueue(nil)
		}
	case InsufficientPlayers:
	default:
		// do nothing, method called incorrectly
		g.log.WithField("game_state", g.String()).Info("no caught state in NextGame call")
		return ErrGameInProgress
	}

	defer g.update()
	g.stopTimeout()
	if g.O != nil && g.X != nil {
		g.setTimeout(g.timeout)
	}
	g.updateStatus()
	return nil
}

// playerTurnId returns the id of the current player awaiting a move, else
// nil
func (g *Game) playerTurnId() *string {
	if g.Status != InProgress {
		return nil
	}
	if g.Move == "X" {
		return &g.X.ID
	}
	if g.Move == "O" {
		return &g.O.ID
	}
	return nil
}

func (g *Game) firstOpenPositionsOnBoard() (x, y int, err error) {
	for y = range g.Board {
		for x = range g.Board[y] {
			if g.Board[y][x] == 0 {
				return x, y, nil
			}
		}
	}
	return -1, -1, errors.New("no empty spots")
}

func (g *Game) stopTimeout() { go func() { g.stopTimeoutCh <- struct{}{} }() }

func (g *Game) resetTimeout() { go func() { g.resetTimeoutCh <- struct{}{} }() }

// setTimeouts job is to make a random move after d duration if none
// has been made, advancing the game. If resetTimeout is activated,
// the loop starts over. If stopTimeout is called, setTimeout(d) must
// be called once more to start the loop again
func (g *Game) setTimeout(d time.Duration) {
	g.log.Info("starting timeout")
	go func() {
		for {
			select {
			case <-time.After(d):
				g.log.Info("timeout received")
				// took too long, timeout and make a random move
				id := g.playerTurnId()
				if id == nil {
					g.log.
						WithError(errors.New("could not find current player id")).
						Error("unable to make automatic move")
					continue
				}
				x, y, err := g.firstOpenPositionsOnBoard()
				if err != nil {
					g.log.WithError(err).Error("unable to calculate random move")
					continue
				}

				g.log.WithFields(logrus.Fields{
					"x":  x,
					"y":  y,
					"id": *id,
				}).Info("placing move for user")

				if err := g.PlacePiece(Move{
					PlayerID: *id,
					XAxis:    x, YAxis: y,
				}); err != nil {
					g.log.WithError(err).Error("unable to place random move")
				}

			case <-g.resetTimeoutCh:
				g.log.Info("resetting timeout")
				continue

			case <-g.stopTimeoutCh:
				g.log.Info("stopping timeout")
				return
			}
		}
	}()
}

func (g *Game) updateStatus() {
	g.Status = g.status()
}

// PlacePiece places p at xLoc/yLoc on board
func (g *Game) PlacePiece(move Move) error {
	logCtx := g.log.WithFields(logrus.Fields{
		"x":         move.XAxis,
		"y":         move.YAxis,
		"player_id": move.PlayerID,
	})
	switch g.Status {
	case XWins, OWins, NoBoard, Cats, InsufficientPlayers:
		logCtx.WithField("status", g.Status).Error("invalid move")
		return ErrInvalidMove
	}

	defer g.update()

	switch g.Move {
	case "X":
		if g.X.ID != move.PlayerID {
			logCtx.Error("not players turns to move")
			return ErrInvalidMove
		}
		if g.Board[move.YAxis][move.XAxis] != blank {
			logCtx.Error("spot already used")
			return ErrInvalidMove
		}
		g.Board[move.YAxis][move.XAxis] = xPiece
		logCtx.WithField("move", g.Move).Info("move placed")
		g.Move = "O"
		g.resetTimeout()

	case "O":
		if g.O.ID != move.PlayerID {
			logCtx.Error("not players turns to move")
			return ErrInvalidMove
		}
		if g.Board[move.YAxis][move.XAxis] != blank {
			logCtx.Error("spot already used")
			return ErrInvalidMove
		}
		g.Board[move.YAxis][move.XAxis] = oPiece
		logCtx.WithField("move", g.Move).Info("move placed")
		g.Move = "X"
		g.resetTimeout()

	default:
	}

	g.updateStatus()
	switch g.Status {
	case XWins, OWins, Cats:
		logCtx.Info("game over, refreshing board")
		go func() {
			<-time.After(3 * time.Second)
			if err := g.NextGame(); err != nil {
				logCtx.Errorf("error advancing: %s", err)
			} else {
				logCtx.WithField("status", g.Status.String()).Info("board refreshed")
			}
		}()
	}
	return nil
}

// AddPlayer adds a player to an empty position, or the bottom of the queue
func (g *Game) AddPlayer(p Player) error {
	defer g.update()
	logCtx := g.log.WithField("player_id", p.ID)

	for _, queued := range g.Queue {
		if p.ID == queued.ID {
			logCtx.Error("player already registered")
			return errors.New("already queued")
		}
	}

	if (g.X != nil && g.X.ID == p.ID) || (g.O != nil && g.O.ID == p.ID) {
		logCtx.Error("player already playing")
		return errors.New("already playing")
	}

	startGame := func() {
		// declaring function here for logCtx
		logCtx.Info("game starting")
		g.Status = InProgress
		g.setTimeout(g.timeout)
	}

	if g.X == nil {
		logCtx.Info("player placed as player X")
		g.X = &p
		if g.O == nil {
			g.Move = "X"
		} else {
			startGame()
		}
	} else if g.O == nil {
		logCtx.Info("player placed as player O")
		g.O = &p
		if g.X == nil {
			g.Move = "O"
		} else {
			startGame()
		}
	} else {
		logCtx.Info("player placed in queue")
		g.Queue = append(g.Queue, p)
	}
	return nil
}

// UpdatePlayer sets or clears the player
func (g *Game) UpdatePlayer(p Player) error {
	defer g.update()
	g.log.WithField("id", p.ID).WithField("name", p.Name).Info("Updating player")
	if g.X != nil && g.X.ID == p.ID {
		g.X = &p
		g.log.Info("Updated X")
		return nil
	}

	if g.O != nil && g.O.ID == p.ID {
		g.O = &p
		g.log.Info("Updated O")
		return nil
	}
	for i := range g.Queue {
		if g.Queue[i].ID == p.ID {
			g.Queue[i] = p
			g.log.Infof("Updated queue position %v", i)
			return nil
		}
	}
	g.log.WithField("id", p.Name).Error("could not find player ID")
	return ErrPlayerNotFound
}

// RemovePlayer removes a player from the queue and returns a 'ErrPlayerNotFound'
// error if no player with the supplied ID was found
func (g *Game) RemovePlayer(id string) error {
	defer g.update()
	logCtx := g.log.WithField("player_id", id)
	if g.X != nil && g.X.ID == id {
		g.X = nil
		logCtx.Info("player removed from position X")
		g.clearBoard()
		return g.NextGame()
	}

	if g.O != nil && g.O.ID == id {
		g.O = nil
		logCtx.Info("player removed from position O")
		g.clearBoard()
		return g.NextGame()
	}

	idx := -1
	for i := range g.Queue {
		if g.Queue[i].ID == id {
			idx = i
			break
		}
	}

	if idx == -1 {
		logCtx.Error("player not playing or in queue")
		return ErrPlayerNotFound
	}

	g.Queue = append(g.Queue[:idx], g.Queue[idx+1:]...)
	logCtx.Info("player removed queue")
	return nil
}

func checkScore(s int) *string {
	switch s {
	case oWins:
		return OWins.ptr()
	case xWins:
		return XWins.ptr()
	}
	return nil
}

func (g *Game) status() Status {
	if g.X == nil || g.O == nil {
		return InsufficientPlayers
	}

	if g.Board == nil {
		return NoBoard
	}

	// if the board total is 0, it is X's move (if there are spots left)
	// else, it is Y's move
	boardTotal := 0

	// if movesLeft is 0, game is over
	movesLeft := 9

	colSums := [...]int{0, 0, 0}

	for _, row := range g.Board {
		rowSum := 0
		for i, col := range row {
			if col != 0 {
				movesLeft--
			}
			rowSum += int(col)
			colSums[i] += int(col)
		}

		boardTotal += rowSum
		if s := checkScore(boardTotal); s != nil {
			return Status(*s)
		}
	}

	for _, col := range colSums {
		if s := checkScore(col); s != nil {
			return Status(*s)
		}
	}

	diagonal := g.Board[0][0] + g.Board[1][1] + g.Board[2][2]
	if s := checkScore(int(diagonal)); s != nil {
		return Status(*s)
	}

	diagonal = g.Board[2][0] + g.Board[1][1] + g.Board[0][2]
	if s := checkScore(int(diagonal)); s != nil {
		return Status(*s)
	}

	if movesLeft == 0 {
		return Cats
	}

	return InProgress
}
