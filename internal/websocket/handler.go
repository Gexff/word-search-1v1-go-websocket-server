package websocket

import(
	"log"
	"net/http"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/Gexff/word-search-1v1-go-websocket-server/internal/server"
	"encoding/json"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	server *server.Server
	routes map[string]func(*server.Player, json.RawMessage)
}

func New(s *server.Server) *Handler {
	h := &Handler{
		server: s,
		routes: make(map[string]func(*server.Player, json.RawMessage)),
	}

	h.routes["create_room"] = h.handleCreateRoom
	h.routes["join_room"]   = h.handleJoinRoom
	h.routes["name_change"] = h.handleNameChange
	h.routes["set_ready"] 	= h.handleSetReady
	h.routes["select_word"] = h.handleSelectWord
	h.routes["set_word_count"] = h.handleSetWordCount
	h.routes["set_grid_size"] = h.handleSetGridSize
	h.routes["leave_room"] = h.handleLeaveRoom
	h.routes["ping"]        = h.handlePing

	return h
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	player := &server.Player{
		ID: uuid.NewString(),
		Conn: conn,
		Send: make(chan interface{}, 16),
	}

	h.server.AddPlayer(player)
	go player.WritePump() // Start player write pump
	defer func() { // cleanup
		close(player.Send)
		h.server.RemovePlayer(player)
	}()

	h.ReadPump(player)
}

func (h *Handler) ReadPump(player *server.Player) {
	for {
		var msg Message
		if err := player.Conn.ReadJSON(&msg); err != nil {
			log.Println("read error:", err)
			return
		}

		handler, ok := h.routes[msg.Type]
		if !ok {
			player.Send <- errorMessage("unknown message type")
			continue
		}

		handler(player, msg.Payload)
	}
}

func (h *Handler) handleCreateRoom(player *server.Player, payload json.RawMessage) {
	roomID := uuid.NewString()

	room, err := h.server.CreateRoom(player, roomID)
	if err != nil {
		player.Send <- errorMessage(err.Error())
		return
	}

	var data struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		player.Send <- errorMessage("invalid payload")
		return
	}

	player.Name = data.Name
	player.Send <- map[string]interface{}{
		"type": "room_created",
		"payload": map[string]interface{}{
			"code": room.JoinCode,
			"player1_name": room.Player1.Name,
			"options": map[string]interface{}{
				"grid_size": room.GameState.GridSize,
				"word_count": room.GameState.WordCount,
			},
		},
	}
}

func (h *Handler) handleJoinRoom(player *server.Player, payload json.RawMessage) {
	var data struct {
		JoinCode string `json:"join_code"`
		Name string `json:"name"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		player.Send <- errorMessage("invalid payload")
		return
	}

	room, err := h.server.JoinRoomByCode(player, data.JoinCode)
	if err != nil {
		player.Send <- errorMessage(err.Error())
		return
	}

	player.Name = data.Name
	log.Println("player joined room. player: ", player.ID, " name: ", data.Name, "room: ", room.ID)

	room.Broadcast(map[string]interface{}{
		"type": "player_joined",
		"payload": map[string]interface{}{
			"player1_name": room.Player1.Name,
			"player2_name": room.Player2.Name,
			"player1_ready": room.PlayerReady[0],
			"player2_ready": room.PlayerReady[1],
			"code": room.JoinCode,
			"options": map[string]interface{}{
				"grid_size": room.GameState.GridSize,
				"word_count": room.GameState.WordCount,
			},
		},
	})
}

func (h *Handler) handleSelectWord(player *server.Player, payload json.RawMessage) {
	var data struct {
		Start server.Coord `json:"start"`
		End   server.Coord `json:"end"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		player.Send <- errorMessage("invalid selection")
		return
	}

	room := player.Room
	if room == nil {
		player.Send <- errorMessage("not in a game")
		return
	}

	if msg, err := room.GameState.ClaimWord(player, data.Start, data.End, room); err != nil {
		player.Send <- errorMessage(err.Error())
	} else {
		room.Broadcast(msg)

		msg, gameOver := room.GameState.CheckForWinner(room)
		if gameOver {
			room.Broadcast(msg)
			room.PlayerReady = [2]bool{false, false}
		}
	}
}

func (h *Handler) handleSetReady(player *server.Player, payload json.RawMessage) {
	var data struct {
		Ready bool `json:"ready"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		player.Send <- errorMessage("invalid selection")
		return
	}

	room := player.Room
	if room == nil {
		player.Send <- errorMessage("not in a game")
		return
	}

	if room.GameState != nil && room.GameState.GameStarted {
		player.Send <- errorMessage("game already started")
		return
	}

	room.SetReady(player, data.Ready)
	room.Broadcast(map[string]interface{}{
		"type": "ready_update",
		"payload": map[string]interface{}{
			"player1_ready": room.PlayerReady[0],
			"player2_ready": room.PlayerReady[1],
		},
	})

	start := room.CheckStartCondition()
	if start {
		room.Broadcast(room.GameState.StartGame())
	}
}

func (h *Handler) handleSetWordCount(player *server.Player, payload json.RawMessage) {
	var data struct {
		WordCount int `json:"word_count"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		player.Send <- errorMessage("invalid selection")
		return
	}

	room := player.Room
	if room == nil {
		player.Send <- errorMessage("not in a game")
		return
	}
	
	if room.Player1.ID != player.ID || player.Number != 1 {
		player.Send <- errorMessage("only Player 1 can modify game settings")
		return
	}

	if room.GameState != nil && room.GameState.GameStarted {
		player.Send <- errorMessage("game already started")
		return
	}

	room.GameState.WordCount = data.WordCount
	room.Broadcast(map[string]interface{}{
		"type": "game_settings",
		"payload": map[string]interface{}{
			"options": map[string]interface{}{
				"grid_size": room.GameState.GridSize,
				"word_count": room.GameState.WordCount,
			},
		},
	})
}

func (h *Handler) handleSetGridSize(player *server.Player, payload json.RawMessage) {
	var data struct {
		GridSize int `json:"grid_size"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		player.Send <- errorMessage("invalid selection")
		return
	}

	if data.GridSize <= 10 {
		player.Send <- errorMessage("insufficient grid size. must be greater than 10.")
		return
	}

	room := player.Room
	if room == nil {
		player.Send <- errorMessage("not in a game")
		return
	}
	
	if room.Player1.ID != player.ID || player.Number != 1 {
		player.Send <- errorMessage("only Player 1 can modify game settings")
		return
	}

	if room.GameState != nil && room.GameState.GameStarted {
		player.Send <- errorMessage("game already started")
		return
	}

	room.GameState.GridSize = data.GridSize
	room.Broadcast(map[string]interface{}{
		"type": "game_settings",
		"payload": map[string]interface{}{
			"options": map[string]interface{}{
				"grid_size": room.GameState.GridSize,
				"word_count": room.GameState.WordCount,
			},
		},
	})
}

func (h *Handler) handleLeaveRoom(player *server.Player, _ json.RawMessage) {
	h.server.RemovePlayerFromRoom(player)
}

func (h *Handler) handleNameChange(player *server.Player, payload json.RawMessage) {
	var data struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		player.Send <- errorMessage("invalid name change")
		return
	}

	player.Name = data.Name

	player.Send <- map[string]interface{}{
		"type": "name_change_accepted",
		"payload": map[string]string{
			"name": player.Name,
		},
	}
	
}

func (h *Handler) handlePing(player *server.Player, _ json.RawMessage) {
	player.Send <- map[string]interface{}{
		"type": "pong",
	}
}

func errorMessage(msg string) map[string]interface{} {
	return map[string]interface{}{
		"type": "error",
		"payload": map[string]string{
			"message": msg,
		},
	}
}
