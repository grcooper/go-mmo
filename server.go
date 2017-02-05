package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pborman/uuid"
)

type Message struct {
	Y      int    // current y position
	X      int    // curreny x position
	Id     string // Id of the player that sent the message
	New    bool   // True if this player just connected
	Online bool   // true if the player is no longer connected so the frontend will remove it's sprite
}

type Player struct {
	Y      int             // Y position of the Player
	X      int             // X Position
	Id     string          // Unique id to identify the Player
	Socket *websocket.Conn // Websocket connection of the player
}

func (p *Player) position(new bool) Message {
	return Message{X: p.X, Y: p.Y, Id: p.Id, New: new, Online: true}
}

// Slice of the *Players which will store list of connected Players
var Players = make([]*Player, 0)

func remoteHandler(res http.ResponseWriter, req *http.Request) {
	var err error

	log.Println("Hit Handler")

	// when someone requires a ws connection we create a new player
	// Store the pointer to the connection inside player.Socket
	ws, err := websocket.Upgrade(res, req, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(res, "Not a websocket handshake", http.StatusBadRequest)
		return
	} else if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Got websocket conn from %v\n", ws.RemoteAddr())
	player := new(Player)
	player.Id = uuid.New()
	player.Socket = ws

	// we broadcast the position of hte new player to already connected Players

	log.Println("Publishing positions")

	go func() {
		for _, p := range Players {
			if p.Socket.RemoteAddr() != player.Socket.RemoteAddr() {
				if err = player.Socket.WriteJSON(p.position(true)); err != nil {
					log.Println(err)
				}
				if err = p.Socket.WriteJSON(player.position(true)); err != nil {
					log.Println(err)
				}
			}
		}
	}()

	Players = append(Players, player)

	for {
		// Handling network errors
		// Other players need to know if the sprite dissapears
		// Remove from the Slice
		if err = player.Socket.ReadJSON(&player); err != nil {
			log.Println("Player Disconnected waiting", err)
			for i, p := range Players {
				if p.Socket.RemoteAddr() == player.Socket.RemoteAddr() {
					Players = append(Players[:i], Players[i+1:]...)
				} else {
					log.Println("Destroy player", player)
					if err = p.Socket.WriteJSON(Message{Online: false, Id: player.Id}); err != nil {
						log.Println(err)
					}
				}
			}
			log.Println("Number of players still connected ...", len(Players))
			return
		}

		// regular broadcast to inform all the players about a player's position
		go func() {
			for _, p := range Players {
				if p.Socket.RemoteAddr() != player.Socket.RemoteAddr() {
					if err = p.Socket.WriteJSON(player.position(false)); err != nil {
						log.Println(err)
					}
				}
			}
		}()
	}
}

func main() {
	log.Println("In main")

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	log.Printf("Got Port: %s\n", port)
	r := mux.NewRouter()
	r.HandleFunc("/ws", remoteHandler)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))
	log.Println("Listen and Serve")
	http.ListenAndServe(port, r)
	log.Println("End of Main")
}
