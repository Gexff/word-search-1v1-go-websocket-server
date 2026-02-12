package server

import (
	"sync"
	"github.com/gorilla/websocket"
)

type Player struct {
	ID string
	Name string
	Number int 		// 1 or 2

	Conn *websocket.Conn
	Send chan interface{}

	Room *Room

	mu sync.Mutex
}

// Write to Send channel, then Player goroutine will write to socket
func (p *Player) WritePump() {
	defer p.Conn.Close()

	for msg := range p.Send {
		if err := p.Conn.WriteJSON(msg); err != nil {
			// Socket Error
			break
		}
	}
}

func (p *Player) Disconnect() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Send != nil {
		close(p.Send)
		p.Send = nil
	}

	if p.Conn != nil {
		p.Conn.Close()
		p.Conn = nil
	}
}