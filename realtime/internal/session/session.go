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
	AnonID    string
	Conn      *websocket.Conn
	Send      chan []byte
	Ctx       context.Context
	Cancel    context.CancelFunc
	mu        sync.Mutex
	CreatedAt time.Time

	TextID           string // ID текста из Python API
	Mode             string
	Text             string // Эталонный текст (получен от Python)
	TextRunes        []rune
	Position         int                // Текущая позиция в тексте (какой символ печатаем)
	Errors           int                // Количество ошибок
	StartTime        time.Time          // Когда начался забег
	HasKeystrokes    bool               // Было ли хотя бы одно нажатие
	FirstKeystrokeAt int64              // Метка первого нажатия
	LastKeystrokeAt  int64              // Метка последнего нажатия
	Keystrokes       []domain.Keystroke // Буфер нажатий для античита и агрегатов

	Validator ValidatorIF
}

type ValidatorIF interface {
	ValidateKS(sess *Session, char string, timestamp int64) domain.ValidationResult
}

type KeystrokePayload struct {
	Char      string `json:"char"`
	T         int64  `json:"t"`
	Timestamp int64  `json:"timestamp"`
}

func NewSession(conn *websocket.Conn, userID int64, isAnon bool, anonID string, Validator ValidatorIF) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		ID:        generateID(),
		UserID:    userID,
		IsAnon:    isAnon,
		AnonID:    anonID,
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
func (s *Session) StartSession(textID, mode, textContent string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TextID = textID
	s.Mode = mode
	s.Text = textContent
	s.TextRunes = []rune(textContent)
	s.Position = 0
	s.Errors = 0
	s.StartTime = time.Now().UTC()
	s.HasKeystrokes = false
	s.FirstKeystrokeAt = 0
	s.LastKeystrokeAt = 0
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

	interval := int64(0)
	if s.HasKeystrokes {
		interval = timestamp - s.LastKeystrokeAt
	}

	if !s.HasKeystrokes {
		s.FirstKeystrokeAt = timestamp
		s.HasKeystrokes = true
	}
	s.LastKeystrokeAt = timestamp

	ks := domain.Keystroke{
		Char:      char,
		Timestamp: timestamp,
		Interval:  interval,
	}

	if char == "Backspace" {
		if s.Position > 0 {
			s.Position--
		}
	} else {
		// take first rune of input char
		var r rune
		for _, ru := range []rune(char) {
			r = ru
			break
		}

		expected := s.TextRunes[s.Position]
		isError := r != expected
		ks.IsError = isError

		if isError {
			s.Errors++
		}
		s.Position++
	}

	s.Keystrokes = append(s.Keystrokes, ks)

	progress := map[string]interface{}{
		"type":      "session.progress",
		"t":         timestamp,
		"position":  s.Position,
		"errors":    s.Errors,
		"completed": s.Position >= len(s.TextRunes),
	}

	return progress, nil
}

// FinishSession computes results and returns client payload and RunResult for API.
func (s *Session) FinishSession() (map[string]interface{}, *apiclient.RunResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	durationMs := int64(0)
	if s.HasKeystrokes {
		durationMs = s.LastKeystrokeAt - s.FirstKeystrokeAt
	}

	typed := 0
	correct := 0
	for _, keystroke := range s.Keystrokes {
		if keystroke.Char == "Backspace" {
			continue
		}

		typed++
		if !keystroke.IsError {
			correct++
		}
	}

	errors := s.Errors

	minutes := float64(durationMs) / 60000.0
	if minutes <= 0 {
		minutes = 1.0 / 60.0
	}

	wpmNet := 0.0
	if correct > 0 {
		wpmNet = (float64(correct) / 5.0) / minutes
	}

	wpmRaw := 0.0
	if typed > 0 {
		wpmRaw = (float64(typed) / 5.0) / minutes
	}

	accuracy := 0.0
	if typed > 0 {
		accuracy = (float64(correct) / float64(typed)) * 100.0
	}

	keystrokes := make([]apiclient.RunKeystroke, 0, len(s.Keystrokes))
	for _, keystroke := range s.Keystrokes {
		keystrokes = append(keystrokes, apiclient.RunKeystroke{
			Char:    keystroke.Char,
			T:       keystroke.Timestamp,
			IsError: keystroke.IsError,
		})
	}

	flags := collectRunFlags(s.Keystrokes)

	clientPayload := map[string]interface{}{
		"type":        "session.result",
		"session_id":  s.ID,
		"wpm_net":     wpmNet,
		"wpm_raw":     wpmRaw,
		"accuracy":    accuracy,
		"duration_ms": durationMs,
		"typed":       typed,
		"correct":     correct,
		"errors":      errors,
	}

	var userID *int64
	var anonID *string
	if s.IsAnon {
		if s.AnonID != "" {
			value := s.AnonID
			anonID = &value
		}
	} else {
		value := s.UserID
		userID = &value
	}

	run := &apiclient.RunResult{
		SessionID:  s.ID,
		UserID:     userID,
		AnonID:     anonID,
		TextID:     s.TextID,
		Mode:       s.Mode,
		StartedAt:  s.StartTime,
		DurationMs: durationMs,
		WPMNet:     wpmNet,
		WPMRaw:     wpmRaw,
		Accuracy:   accuracy,
		Typed:      typed,
		Correct:    correct,
		Errors:     errors,
		Flags:      flags,
		Keystrokes: keystrokes,
	}

	return clientPayload, run
}

func collectRunFlags(keystrokes []domain.Keystroke) []string {
	flags := make([]string, 0)
	nonMonotonic := false
	belowMinimum := false

	for i := 1; i < len(keystrokes); i++ {
		interval := keystrokes[i].Timestamp - keystrokes[i-1].Timestamp
		if interval <= 0 {
			nonMonotonic = true
		}
		if interval > 0 && interval < 15 {
			belowMinimum = true
		}
	}

	if nonMonotonic {
		flags = append(flags, "non_monotonic_time")
	}
	if belowMinimum {
		flags = append(flags, "interval_below_minimum")
	}

	return flags
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (s *Session) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
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

	s.Conn.SetReadLimit(65536)
	s.Conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	s.Conn.SetPongHandler(func(string) error {
		s.Conn.SetReadDeadline(time.Now().Add(75 * time.Second))
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
