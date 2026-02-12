package server

import (
	"sync"
	"time"
	"math/rand"
	"github.com/Gexff/word-search-1v1-go-websocket-server/internal/game"
	"errors"
	"log"
)

type Server struct {
	mu sync.Mutex
	players map[string]*Player
	rooms map[string]*Room	// roomID -> room
	codes map[string]*Room // JoinCode-> room
	wordlist []string
}

const wordlistPath = "config/words.txt"

func New() *Server {
	rand.Seed(time.Now().UnixNano())
	
	return &Server {
		players: make(map[string]*Player),
		rooms: make(map[string]*Room),
		codes: make(map[string]*Room),
		wordlist: game.ReadWords(wordlistPath),
	}
}

func (s *Server) AddPlayer(p *Player) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.players[p.ID] = p
}

func (s *Server) RemovePlayer(player *Player) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.players, player.ID)

	if room := player.Room; room != nil {
		room.RemovePlayer(player)

		if room.IsEmpty() {
			log.Println("deleting room: ", room)
			delete(s.rooms, room.ID)
			delete(s.codes, room.JoinCode)
		}
	}
}

func (s *Server) RemovePlayerFromRoom(player *Player) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if room := player.Room; room != nil {
		room.RemovePlayer(player)

		if room.IsEmpty() {
			log.Println("deleting room: ", room)
			delete(s.rooms, room.ID)
			delete(s.codes, room.JoinCode)
		}
	}

	player.Room = nil
}

func (s *Server) CreateRoom(owner *Player, roomID string) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if (owner.Room != nil) {
		return nil, errors.New("player already in room")
		log.Println("failed to create room, player already in room. player: ", owner.ID)
	}

	var code string
    for {
        code = generateCode()
        if _, exists := s.codes[code]; !exists {
            break
        }
    }

	room := &Room{
		ID: roomID,
		JoinCode: code,
		Player1: owner,
		PlayerReady: [2]bool{false, false},
		GameState: &GameState{
			wordlist: s.wordlist,
			WordCount: 7,	// Default settings
			GridSize: 12,
		},
	}

	s.rooms[room.ID] = room
	s.codes[room.JoinCode] = room
	
	owner.Room = room
	owner.Number = 1

	log.Println("room created: ", room.ID, " JoinCode: ", room.JoinCode, " owner: ", owner.ID)

	return room, nil
}

func (s *Server) JoinRoomByCode(p *Player, code string) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, exists := s.codes[code]
	if !exists {
		log.Println("failed to join room. requesting player: ", p.ID, " code: ", code)
		return nil, errors.New("invalid room code")
	}

	if room.Player2 != nil {
		return nil, errors.New("room is full")
	}

	room.Player2 = p
	p.Room = room
	p.Number = 2
	return room, nil
}

const codeLength = 6
const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateCode() string {
	b := make([]byte, codeLength)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}