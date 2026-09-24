package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/emount4/typing-realtime/internal/apiclient"
	"github.com/emount4/typing-realtime/internal/auth"
	"github.com/emount4/typing-realtime/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Verifier interface {
	Verify(tokenString string) (*auth.JWTClaims, error)
}

// APIClient is the interface consumers in transport use to talk to Python API.
type APIClient interface {
	GetText(ctx context.Context, textID string) (*apiclient.TextResponse, error)
	SubmitRun(ctx context.Context, run *apiclient.RunResult) (*apiclient.RunSubmitResult, error)
}

type WebSocketHandler struct {
	upgrader  websocket.Upgrader
	verifier  Verifier
	sessions  map[string]*session.Session
	mu        sync.Mutex
	apiClient APIClient
}

func NewWebSocketHandler(verifier Verifier, client APIClient) *WebSocketHandler {
	return &WebSocketHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		verifier:  verifier,
		sessions:  make(map[string]*session.Session),
		apiClient: client,
	}
}

type keystrokeEvent struct {
	Char      string
	Timestamp int64
}

type keystrokeBatchMessage struct {
	Items []struct {
		Char      string `json:"char"`
		T         *int64 `json:"t"`
		Timestamp *int64 `json:"timestamp"`
	} `json:"items"`
	Data []session.KeystrokePayload `json:"data"`
}

func (h *WebSocketHandler) Echo(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade to WebSocket"})
		return
	}
	defer conn.Close()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if err := conn.WriteMessage(messageType, message); err != nil {
			break
		}
	}
}

func (h *WebSocketHandler) HandleWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	// expect auth frame within 5s
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Printf("auth timeout or error: %v", err)
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{})

	var authMsg AuthMessage
	var userID int64
	var isAnon bool
	var anonID string

	if err := json.Unmarshal(message, &authMsg); err != nil || authMsg.Type != "auth" {
		isAnon = true
	} else if authMsg.Token != nil && *authMsg.Token != "" {
		claims, err := h.verifier.Verify(*authMsg.Token)
		if err != nil {
			log.Printf("token verification failed: %v", err)
			isAnon = true
		} else {
			userID = claims.UserID
			isAnon = false
		}
	} else {
		isAnon = true
	}
	if isAnon && authMsg.AnonID != nil {
		anonID = *authMsg.AnonID
	}

	validator := session.NewValidaator()
	sess := session.NewSession(conn, userID, isAnon, anonID, validator)
	h.mu.Lock()
	h.sessions[sess.ID] = sess
	h.mu.Unlock()

	log.Printf("session created: %s, user: %d, anon: %v", sess.ID, userID, isAnon)

	go sess.WritePump()
	go sess.ReadPump(h.messageHandler)
}

func (h *WebSocketHandler) messageHandler(sess *session.Session, msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		log.Printf("message without type: %v", msg)
		return
	}

	switch msgType {
	case "auth":
		log.Printf("unexpected auth from session %s", sess.ID)

	case "session.start":
		textID, ok := msg["text_id"].(string)
		if !ok {
			return
		}
		mode, ok := msg["mode"].(string)
		if !ok || mode != "solo" {
			return
		}
		text, err := h.apiClient.GetText(context.Background(), textID)
		if err != nil {
			log.Printf("failed to get text %s: %v", textID, err)
			return
		}
		sess.StartSession(textID, mode, text.Content)
		response := map[string]interface{}{
			"type":            "session.started",
			"session_id":      sess.ID,
			"text_id":         textID,
			"text_length":     len(sess.TextRunes),
			"allow_backspace": true,
			"server_time_ms":  time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(response)
		sess.Send <- data

	case "keystroke", "keystrokes", "keystroke.batch":
		items, err := parseKeystrokes(msgType, msg)
		if err != nil {
			log.Printf("invalid keystroke payload: %v", err)
			return
		}

		var lastProgress map[string]interface{}
		for _, item := range items {
			res := sess.Validator.ValidateKS(sess, item.Char, item.Timestamp)
			if !res.IsValid {
				log.Printf("invalid keystroke: %s", res.Reason)
				continue
			}

			progress, err := sess.ProcessKeystroke(item.Char, item.Timestamp)
			if err != nil {
				log.Printf("process keystroke error: %v", err)
				continue
			}
			lastProgress = progress
		}

		if lastProgress != nil {
			data, _ := json.Marshal(lastProgress)
			sess.Send <- data
		}

	case "session.finish":
		payload, run := sess.FinishSession()
		data, _ := json.Marshal(payload)
		sess.Send <- data

		go func(r *apiclient.RunResult, s *session.Session) {
			var saveResult *apiclient.RunSubmitResult
			var err error
			if h.apiClient != nil {
				saveResult, err = h.apiClient.SubmitRun(context.Background(), r)
				if err != nil {
					log.Printf("failed to submit run: %v", err)
					return
				}
			} else {
				saveResult = &apiclient.RunSubmitResult{}
			}

			if saveResult != nil {
				savedPayload := map[string]interface{}{
					"type":             "session.saved",
					"session_id":       s.ID,
					"run_id":           saveResult.RunID,
					"is_personal_best": saveResult.IsPersonalBest,
					"flagged":          saveResult.Flagged,
				}
				data, _ := json.Marshal(savedPayload)
				s.Send <- data
			}
			s.Close()
			h.mu.Lock()
			delete(h.sessions, s.ID)
			h.mu.Unlock()
		}(run, sess)

	default:
		log.Printf("unknown message type: %s", msgType)
	}
}

func parseKeystrokes(msgType string, msg map[string]interface{}) ([]keystrokeEvent, error) {
	if msgType == "keystrokes" || msgType == "keystroke.batch" {
		payloadBytes, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}

		var batch keystrokeBatchMessage
		if err := json.Unmarshal(payloadBytes, &batch); err != nil {
			return nil, err
		}

		items := make([]keystrokeEvent, 0, len(batch.Items)+len(batch.Data))
		for _, item := range batch.Items {
			timestamp := int64(0)
			switch {
			case item.T != nil:
				timestamp = *item.T
			case item.Timestamp != nil:
				timestamp = *item.Timestamp
			default:
				return nil, fmt.Errorf("keystroke item without timestamp")
			}

			items = append(items, keystrokeEvent{Char: item.Char, Timestamp: timestamp})
		}
		for _, item := range batch.Data {
			timestamp := item.Timestamp
			if timestamp == 0 {
				timestamp = item.T
			}
			items = append(items, keystrokeEvent{Char: item.Char, Timestamp: timestamp})
		}

		return items, nil
	}

	char, ok := msg["char"].(string)
	if !ok {
		return nil, fmt.Errorf("keystroke without char")
	}

	timestamp := int64(0)
	if ts, ok := msg["t"].(float64); ok {
		timestamp = int64(ts)
	} else if ts, ok := msg["timestamp"].(float64); ok {
		timestamp = int64(ts)
	} else {
		return nil, fmt.Errorf("keystroke without timestamp")
	}

	return []keystrokeEvent{{Char: char, Timestamp: timestamp}}, nil
}
