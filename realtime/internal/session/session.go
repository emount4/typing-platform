package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

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
}

func NewSession(conn *websocket.Conn, userID int64, isAnon bool) *Session {
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
	}
}

func (s *Session) Close() {
	s.Cancel()
	close(s.Send)
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
