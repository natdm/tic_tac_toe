# Tic Tac Toe in Go

## Decription:
Simple tic tac toe server that accepts multiple players and saves game state to firebase. 
* Winner plays again, loser goes to the bottom of the queue
* 5 seconds to make your move
* Games ending in Cats select a random winner
* If you won your last game, you do not go first on the next game

## Setup:
```
dep ensure
go run *.go
```

Server runs on :8080 by default

Set up the server by sending the Firebase configuration to POST /init

## Endpoints:
* GET /
  * Gets the game status
* POST /init
  * Sets up the database. No moves are updated to users until then, though moves can be placed (buggy).
* GET /clear
  * Clears the game and board
* POST /player/move
  * takes a move request with a body of `{"player_id": string, "x_axis": number, "y_axis": number}`
* PUT /player/update
  * updates a player. Must have an ID that is already registered. Takes a body of `{"id": string, "name": string}`
* POST /player/subscribe
  * subscribes a user to a game. Takes a POST request of `{"id": string, "name": string}`
* POST /player/unsubscribe takes a request of `{"id": string, "name": string}` to unsubscribe to games to
