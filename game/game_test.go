package game

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
)

func TestGameStatus(t *testing.T) {
	tCases := []struct {
		name     string
		game     Game
		expected Status
	}{
		{
			name: "Returns insufficient players when there are less than 2 players",
			game: Game{
				log:   logrus.WithField("test", true),
				Board: &Board{},
				Queue: []Player{},
			},
			expected: InsufficientPlayers,
		},
		{
			name: "Returns no board if Board is nil and there are players",
			game: Game{
				log:   logrus.WithField("test", true),
				Board: nil,
				Queue: []Player{{}, {}},
				X:     &Player{},
				O:     &Player{},
			},
			expected: NoBoard,
		},
		{
			name: "Returns in progress if no winners",
			game: Game{
				log: logrus.WithField("test", true),
				Board: &Board{
					{blank, blank, blank},
					{blank, blank, blank},
					{blank, blank, blank}},
				Queue: []Player{{}, {}, {}},
				X:     &Player{},
				O:     &Player{},
			},
			expected: InProgress,
		},
		{
			name: "Returns X wins for horizontal",
			game: Game{
				log: logrus.WithField("test", true),
				Board: &Board{
					{xPiece, xPiece, xPiece},
					{oPiece, oPiece, blank},
					{blank, blank, blank}},
				Queue: []Player{{}, {}, {}},
				X:     &Player{},
				O:     &Player{},
			},
			expected: XWins,
		},
		{
			name: "Returns Y wins for horizontal",
			game: Game{
				log: logrus.WithField("test", true),
				Board: &Board{
					{oPiece, oPiece, oPiece},
					{xPiece, xPiece, blank},
					{xPiece, blank, blank}},
				Queue: []Player{{}, {}, {}},
				X:     &Player{},
				O:     &Player{},
			},
			expected: OWins,
		},
		{
			name: "Returns x wins for vertical",
			game: Game{
				log: logrus.WithField("test", true),
				Board: &Board{
					{xPiece, oPiece, oPiece},
					{xPiece, oPiece, blank},
					{xPiece, blank, blank}},
				Queue: []Player{{}, {}, {}},
				X:     &Player{},
				O:     &Player{},
			},
			expected: XWins,
		},
		{
			name: "Returns x wins for horizontal",
			game: Game{
				Board: &Board{
					{xPiece, xPiece, xPiece},
					{oPiece, oPiece, blank},
					{blank, blank, blank}},
				Queue: []Player{{}, {}, {}},
				X:     &Player{},
				O:     &Player{},
				log:   logrus.WithField("test", true),
			},
			expected: XWins,
		},
		{
			name: "Returns x wins for diagonal",
			game: Game{
				Board: &Board{
					{xPiece, oPiece, xPiece},
					{oPiece, xPiece, blank},
					{blank, blank, xPiece}},
				Queue: []Player{{}, {}, {}},
				X:     &Player{},
				O:     &Player{},
				log:   logrus.WithField("test", true),
			},
			expected: XWins,
		},
		{
			name: "Returns cats for no winner",
			game: Game{
				log: logrus.WithField("test", true),
				Board: &Board{
					{xPiece, oPiece, xPiece},
					{oPiece, xPiece, oPiece},
					{oPiece, xPiece, oPiece}},
				Queue: []Player{{}, {}, {}},
				X:     &Player{},
				O:     &Player{}},
			expected: Cats,
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("expected: %v\n game status: %v\n", tc.expected, tc.game.status())
			assert.Equal(t, tc.game.status(), tc.expected,
				"expected game status to equal board status")
		})
	}
}

// tstBoardPtr creates a new board pointer out of all the positions p
func tstBoardPtr(p ...Piece) *Board {
	if len(p) != 9 {
		panic("need 9 items in p for tstBoardPtr")
	}

	board := Board([3][3]Piece{
		{p[0], p[1], p[2]},
		{p[3], p[4], p[5]},
		{p[6], p[7], p[8]}},
	)
	return &board
}

func (g *Game) json() string {
	bs, _ := json.MarshalIndent(g, "", " ")
	return string(bs)
}

func TestNextGame(t *testing.T) {
	tCases := []struct {
		name        string
		err         error
		game        *Game
		compareGame func(t *testing.T, actual *Game)
	}{
		{
			name: "advances the next player and puts the loser back",
			err:  nil,
			game: &Game{
				Board: tstBoardPtr(
					-1, -1, -1,
					1, 1, -1,
					-1, 1, 1,
				),
				Status: XWins,
				O:      &Player{ID: "testIDO"},
				X:      &Player{ID: "testIDX"},
				Queue: []Player{
					{ID: "TestIDFoo"},
					{ID: "TestIDBar"},
				},
				log: logrus.WithField("test", true),
			},
			compareGame: func(t *testing.T, actual *Game) {
				if actual.X.ID != "testIDX" {
					t.Error("Expected actual.X.ID to still be in queue")
					t.Fail()
				}
				if actual.O.ID != "TestIDFoo" {
					t.Errorf("Expected actual.O.ID to be TestIDFoo but was %s", actual.O.ID)
					t.Fail()
				}
				if actual.Move != "O" {
					t.Errorf("Expected actual.Move to be O but was %s", actual.Move)
					t.Fail()
				}
				if len(actual.Queue) != 2 {
					t.Errorf("Expected actual.Queue to be 2 len but was %v", len(actual.Queue))
					t.FailNow()
				}
				if actual.Queue[0].ID != "TestIDBar" {
					t.Errorf("Expected actual.Queue[0].ID = TestIDBar but was %s", actual.Queue[0].ID)
					t.Fail()
				}
				if actual.Queue[1].ID != "testIDO" {
					t.Errorf("Expected actual.Queue[1].ID = testIDO but was %s", actual.Queue[1].ID)
					t.Fail()
				}
			},
		}, {
			name: "loser plays again if nobody in queue",
			err:  nil,
			game: &Game{
				Board: tstBoardPtr(
					-1, -1, -1,
					1, 1, -1,
					-1, 1, 1,
				),
				Status: XWins,
				O:      &Player{ID: "testIDO"},
				X:      &Player{ID: "testIDX"},
				Queue:  []Player{},
				log:    logrus.WithField("test", true),
			},
			compareGame: func(t *testing.T, actual *Game) {
				if actual.X.ID != "testIDX" {
					t.Error("Expected actual.X.ID to still be in queue")
					t.Fail()
				}
				if actual.O.ID != "testIDO" {
					t.Errorf("Expected actual.O.ID to be testIDO but was %s", actual.O.ID)
					t.Fail()
				}
				if actual.Move != "O" {
					t.Errorf("Expected actual.Move to be O but was %s", actual.Move)
					t.Fail()
				}
				if len(actual.Queue) != 0 {
					t.Errorf("Expected actual.Queue to be 0 len but was %v", len(actual.Queue))
					t.FailNow()
				}
			},
		}, {
			name: "one of the players loses on Cats",
			err:  nil,
			game: &Game{
				Board: tstBoardPtr(
					-1, -1, -1,
					1, 1, -1,
					-1, 1, 1,
				),
				Status: Cats,
				O:      &Player{ID: "testIDO"},
				X:      &Player{ID: "testIDX"},
				Queue: []Player{
					{ID: "TestIDFoo"},
					{ID: "TestIDBar"},
				},
				log: logrus.WithField("test", true),
			},
			compareGame: func(t *testing.T, actual *Game) {
				if actual.X.ID == "testIDX" && actual.O.ID == "testIDO" {
					t.Error("Expected one of x or y to drop from the queue")
					t.Fail()
				}
				if actual.O.ID != "testIDO" {
					t.Errorf("Expected actual.O.ID to be TestIDFoo but was %s", actual.O.ID)
					t.Fail()
				}
				if len(actual.Queue) != 2 {
					t.Errorf("Expected actual.Queue to be 0 len but was %v", len(actual.Queue))
					t.FailNow()
				}
				if actual.Queue[0].ID != "TestIDBar" {
					t.Errorf("Expected actual.Queue[0].ID = TestIDBar but was %s", actual.Queue[0].ID)
					t.Fail()
				}
			},
		},
	}

	for i, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tCases[i].game.NextGame()
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
			} else {
				assert.NoError(t, err)
			}
			tc.compareGame(t, tCases[i].game)
		})
	}
}
