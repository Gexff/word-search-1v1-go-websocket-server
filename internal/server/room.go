package server

import (
	"sync"
)

type Room struct {
	ID   string
	JoinCode string

	Player1 *Player
	Player2 *Player

	PlayerReady [2]bool
	GameState *GameState

	mu sync.Mutex
}

func (r *Room) Broadcast(msg interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Player1 != nil {
		safeSend(r.Player1, msg)
	}
	if r.Player2 != nil {
		safeSend(r.Player2, msg)
	}
}

func (r *Room) SendToPlayer(p *Player, msg interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	safeSend(p, msg)
}

func safeSend(p *Player, msg interface{}) {
	select {
	case p.Send <- msg:
		// Sent succesfully
	default:
		// player's send buffer is full → treat as disconnected
		go p.Disconnect()
	}
}

func (r *Room) SetReady(p *Player, state bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Player1.ID == p.ID {
		r.PlayerReady[0] = state
	} else if r.Player2.ID == p.ID {
		r.PlayerReady[1] = state
	} 
}

func (r *Room) CheckStartCondition() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.PlayerReady[0] && r.PlayerReady[1]
}

func (r *Room) RemovePlayer(player *Player){
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Player1 != nil && r.Player1.ID == player.ID {
		r.Player1 = r.Player2
		if r.Player1 != nil {
			r.Player1.Number = 1
		}
		r.Player2 = nil
	} else if r.Player2 != nil && r.Player2.ID == player.ID {
		r.Player2 = nil
	}
	r.PlayerReady[0] = false
	r.PlayerReady[1] = false
	r.GameState.GameStarted = false

	if r.Player1 == nil && r.Player2 == nil {
		return
	}

	msg := map[string]interface{}{
		"type": "player_left",
		"payload": map[string]interface{}{
			"player1_name": r.Player1.Name,
			"player1_ready": r.PlayerReady[0],
			"player2_ready": r.PlayerReady[1],
		},
	}
	safeSend(r.Player1, msg)
}

func (r *Room) IsEmpty() bool {
	return r.Player1 == nil && r.Player2 == nil
}