package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/emount4/typing-realtime/internal/apiclient"
	"github.com/emount4/typing-realtime/internal/domain"
	"github.com/gorilla/websocket"
)

type Session struct {
	ID        string
	UserID    int64
	IsAnon    bool
	Conn      *websocket.Conn
	Send      chan []byte
	Ctx       context.Context
	Cancel    context.CancelFunc
	mu        sync.Mutex
	CreatedAt time.Time

	TextID     string // ID текста из Python API
	Text       string // Эталонный текст (получен от Python)
	TextRunes  []rune
	Position   int                // Текущая позиция в тексте (какой символ печатаем)
	Errors     int                // Количество ошибок
	StartTime  time.Time          // Когда начался забег
	Keystrokes []domain.Keystroke // Буфер нажатий для античита и агрегатов

	Validator ValidatorIF
}

type ValidatorIF interface {
	ValidateKS(sess *Session, char string, timestamp int64) domain.ValidationResult
}

func NewSession(conn *websocket.Conn, userID int64, isAnon bool, Validator ValidatorIF) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		ID:        generateID(),
		UserID:    userID,
		IsAnon:    isAnon,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		Ctx:       ctx,
		Cancel:    cancel,
		CreatedAt: time.Now(),

		Validator: Validator,
	}
}

func (s *Session) Close() {
	s.Cancel()
	close(s.Send)
}

// StartSession initializes session state with the given text.
func (s *Session) StartSession(textID, textContent string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TextID = textID
	s.Text = textContent
	s.TextRunes = []rune(textContent)
	s.Position = 0
	s.Errors = 0
	s.StartTime = time.Now()
	s.Keystrokes = make([]domain.Keystroke, 0)
}

// ProcessKeystroke records a keystroke and updates position/errors.
// Returns a map ready to be sent to client as session.progress.
func (s *Session) ProcessKeystroke(char string, timestamp int64) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Position >= len(s.TextRunes) {
		return nil, fmt.Errorf("text already completed")
	}

	// take first rune of input char
	var r rune
	for _, ru := range []rune(char) {
		r = ru
		break
	}

	expected := s.TextRunes[s.Position]
	isError := r != expected

	interval := int64(0)
	if len(s.Keystrokes) > 0 {
		interval = timestamp - s.Keystrokes[len(s.Keystrokes)-1].Timestamp
	}

	ks := domain.Keystroke{
		Char:      char,
		Timestamp: timestamp,
		IsError:   isError,
		Interval:  interval,
	}
	s.Keystrokes = append(s.Keystrokes, ks)

	if isError {
		s.Errors++
	} else {
		s.Position++
	}

	progress := map[string]interface{}{
		"type":     "session.progress",
		"position": s.Position,
		"length":   len(s.TextRunes),
		"errors":   s.Errors,
	}

	return progress, nil
}

// FinishSession computes results and returns client payload and RunResult for API.
func (s *Session) FinishSession() (map[string]interface{}, *apiclient.RunResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	durationMs := time.Since(s.StartTime).Milliseconds()
	total := len(s.Keystrokes)
	errors := s.Errors
	correct := total - errors

	minutes := float64(durationMs) / 60000.0
	if minutes <= 0 {
		minutes = 1.0 / 60.0
	}

	wpm := 0.0
	if correct > 0 {
		wpm = (float64(correct) / 5.0) / minutes
	}

	accuracy := 0.0
	if total > 0 {
		accuracy = (float64(correct) / float64(total)) * 100.0
	}

	clientPayload := map[string]interface{}{
		"type":             "session.result",
		"wpm":              wpm,
		"accuracy":         accuracy,
		"duration_ms":      durationMs,
		"total_keystrokes": total,
		"errors":           errors,
	}

	run := &apiclient.RunResult{
		UserID:          s.UserID,
		TextID:          s.TextID,
		WPM:             wpm,
		Accuracy:        accuracy,
		DurationMs:      durationMs,
		TotalKeystrokes: total,
		Errors:          errors,
		IsPersonalBest:  false,
	}

	return clientPayload, run
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (s *Session) WritePump() {
	ticker := time.NewTicker(55 * time.Second)
	defer func() {
		s.Conn.Close()
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-s.Send:
			s.mu.Lock()
			if !ok {
				s.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				s.mu.Unlock()
				return
			}
			err := s.Conn.WriteMessage(websocket.TextMessage, message)
			s.mu.Unlock()
			if err != nil {
				return
			}
		case <-ticker.C:
			s.mu.Lock()
			err := s.Conn.WriteMessage(websocket.PingMessage, nil)
			s.mu.Unlock()
			if err != nil {
				return
			}
		case <-s.Ctx.Done():
			return
		}
	}
}

func (s *Session) ReadPump(handler func(session *Session, msg map[string]any)) {
	defer func() {
		s.Cancel()
		s.Conn.Close()
	}()

	s.Conn.SetReadLimit(512)
	s.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	s.Conn.SetPongHandler(func(string) error {
		s.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := s.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("session %s read error: %v", s.ID, err)
			}
			break
		}

		var msg map[string]any
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		handler(s, msg)
	}
}
