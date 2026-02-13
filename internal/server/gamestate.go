package server

import (
	"sync"
	"strings"
	"errors"
	"math/rand"
	"log"
)

type GameState struct {
	Board [][]rune
	Words []string				
	Claimed map[string]string 	// word -> playerID
	wordCoords map[string]WordCoords
	Score [2]int
	GameStarted bool
	wordlist []string
	WordCount int
	GridSize int

	mu sync.Mutex
}

type Coord struct {
	Row, Col int
}

type WordCoords struct {
    Start [2]int `json:"start"`
    End   [2]int `json:"end"`
}

func (g *GameState) getWordFromCoords(start, end Coord) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	dRow := end.Row - start.Row
	dCol := end.Col - start.Col

	lenRow := abs(dRow)
	lenCol := abs(dCol)

	steps := max(lenRow, lenCol)
	if steps == 0 { // both 0, single cell
		return "", errors.New("invalid selection: single cell")
	}

	if lenRow != 0 && lenCol != 0 && lenRow != lenCol { // diagonal selecton, but length != width, so invalid
		return "", errors.New("invalid selection: crooked diagonal")
	}

	stepRow := sign(dRow)
	stepCol := sign(dCol)

	letters := make([]rune, steps+1)
	row, col := start.Row, start.Col
	for i := 0; i <= steps; i++ {
		if row < 0 || row >= len(g.Board) || col < 0 || col >= len(g.Board[0]){
			return "", errors.New("section out of bounds")
		}
		letters[i] = g.Board[row][col]
		row += stepRow
		col += stepCol
	}

	return strings.ToUpper(string(letters)), nil
}

func abs(a int) int {
	return max(a, -a)
}

func sign(x int) int {
	switch {
	case x < 0:
		return -1
	case x > 0:
		return 1
	default:
		return 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func (g *GameState) ClaimWord(player *Player, start, end Coord, r *Room) (interface{}, error) {
	word, err := g.getWordFromCoords(start, end)

	if err != nil {
		return nil, err
	} else {
		log.Println("highlighted word: ", word, " reversed: ", reverse(word))
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.GameStarted {
		return nil, errors.New("game not started")
	}

	// check if the word is in the valid list
	found := false
	for _, w := range g.Words {
		if strings.ToUpper(w) == word {
			found = true
			break
		}
	}
	// Check if reversed word is in the word list
	if !found { 
		word = reverse(word)
		for _, w := range g.Words {
		if strings.ToUpper(w) == word {
			found = true
			break
		}
	}
	}

	if !found {
		return nil, errors.New("invalid word")
	}

	// check if already claimed
	if _, claimed := g.Claimed[word]; claimed {
		return nil, errors.New("word already claimed")
	}

	// claim the word
	g.Claimed[word] = player.ID
	if r.Player1.ID == player.ID {
		g.Score[0] += 1
	} else if r.Player2.ID == player.ID{
		g.Score[1] += 1
	} else {
		return nil, errors.New("invalid player ID on word claim")
	}

	// broadcast to both players
	msg := map[string]interface{}{
		"type": "word_claimed",
		"payload": map[string]interface{}{
			"word": word,
			"player_number": player.Number,
			"start": [2]int{start.Row, start.Col},
			"end": [2]int{end.Row, end.Col},
			"score": g.Score,
		},
	}

	return msg, nil
}

func reverse(s string) string {
    runes := []rune(s)
    for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
        runes[i], runes[j] = runes[j], runes[i]
    }
    return string(runes)
}

func (g *GameState) CheckForWinner(r *Room) (interface{}, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	var msg interface{}
	if g.Score[0] >= len(g.Words)/2 + 1 { // Player 1 wins!
		r.Player1.mu.Lock()
		defer r.Player1.mu.Unlock()

		msg = map[string]interface{}{
			"type": "game_over",
			"payload": map[string]interface{}{
				"winner": r.Player1.Number,
				"unclaimed_words": g.getUnclaimedWordCoords(),
			},
		}
	} else if g.Score[1] >= len(g.Words)/2 + 1 { // Player 2 wins!
		r.Player2.mu.Lock()
		defer r.Player2.mu.Unlock()

		msg = map[string]interface{}{
			"type": "game_over",
			"payload": map[string]interface{}{
				"winner": r.Player2.Number,
				"unclaimed_words": g.getUnclaimedWordCoords(),
			},
		}
	} else {
		return nil, false
	}

	g.GameStarted = false
	return msg, true
}

func (g *GameState) getUnclaimedWordCoords() []WordCoords {
	var unclaimedWords []WordCoords

	for _, word := range g.Words {
		word = strings.ToUpper(word)
        if _, ok := g.Claimed[word]; !ok {
            unclaimedWords = append(unclaimedWords, g.wordCoords[word])
        }
    }

    return unclaimedWords
}


func (g *GameState) StartGame() interface{} {
	g.Claimed = make(map[string]string)
	g.GameStarted = true
	g.Score = [2]int{0, 0}
	g.Words = g.getRandomWords(g.WordCount)
	g.Board = g.generateBoard(g.GridSize, g.Words)

	return map[string]interface{}{
		"type": "game_start",
		"payload": map[string]interface{}{
			"board": g.Board,
			"words": g.Words,
		},
	}
}

func (g *GameState) getRandomWords(n int) []string {
	indices := make([]int, 0, n)
	words := make([]string, 0, n)
	seen := make(map[int]struct{})

	for len(indices) < n {
		num := rand.Intn(len(g.wordlist))
		if _, ok := seen[num]; !ok {
			seen[num] = struct{}{}
			indices = append(indices, num)
		}
	}

	for _, i := range indices {
		words = append(words, g.wordlist[i])
	}

	return words
}

func (g *GameState) generateBoard(GridSize int, words []string) [][]rune {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Initialize Board
	board := make([][]rune, GridSize)
	for i := range board {
		board[i] = make([]rune, GridSize)
		for j := range board[i] {
			board[i][j] = 0 // empty
		}
	}

	// Initialize map
	g.wordCoords = make(map[string]WordCoords, len(words))

	// Place words randomly
	for _, word := range words {
		word = strings.ToUpper(string(word))
		success := false
		attempts := 0

		for !success && attempts < 100 {
			attempts++

			dirRow := rand.Intn(3) - 1 // -1, 0, 1
			dirCol := rand.Intn(3) - 1 // -1, 0, 1

			// skip zero direction
			if dirRow == 0 && dirCol == 0 {
				continue
			}

			// choose random starting cell
			maxRow := GridSize - 1
			maxCol := GridSize - 1

			var startRow int
			var startCol int

			log.Println(maxRow, maxCol, dirRow, dirCol, len(word), word)

			if dirRow == 1 {
				startRow = rand.Intn(maxRow - len(word) + 1)
			} else if dirRow == 0 {
				startRow = rand.Intn(maxRow + 1)
			} else { // dirRow == -1
				startRow = rand.Intn(maxRow - len(word) + 1) + len(word) - 1
			}

			if dirCol == 1 {
				startCol = rand.Intn(maxCol - len(word) + 1)
			} else if dirCol == 0 {
				startCol = rand.Intn(maxCol + 1)
			} else { // dirCol == -1
				startCol = rand.Intn(maxCol - len(word) + 1) + len(word) - 1
			}

			// check if the word fits
			row, col := startRow, startCol
			canPlace := true
			for _, c := range word {
				if row < 0 || row >= GridSize || col < 0 || col >= GridSize {
					canPlace = false
					break
				}
				if board[row][col] != 0 && board[row][col] != c {
					canPlace = false
					break
				}
				row += dirRow
				col += dirCol
			}

			if !canPlace {
				continue
			}

			// place the word
			row, col = startRow, startCol
			startCoord := [2]int{row, col}
			for _, c := range word {
				board[row][col] = c
				row += dirRow
				col += dirCol
			}
			endCoord := [2]int{row-dirRow, col-dirCol}
			g.wordCoords[word] = WordCoords{startCoord, endCoord}

			success = true
		}
		// Possible but very unlikely to not place a word
	}

	// Fill empty cells with random letters
	for i := 0; i < GridSize; i++ {
		for j := 0; j < GridSize; j++ {
			if board[i][j] == 0 {
				board[i][j] = randomLetter()
			}
		}
	}

	return board
}

func randomLetter() rune {
	return rune('A' + rand.Intn(26)) // 26 letters
}