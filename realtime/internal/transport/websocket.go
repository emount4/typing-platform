package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
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
	SubmitRun(ctx context.Context, run *apiclient.RunResult) error
}

type WebSocketHandler struct {
	upgrader  websocket.Upgrader
	verifier  Verifier
	sessions  map[string]*session.Session
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

	if err := json.Unmarshal(message, &authMsg); err != nil || authMsg.Type != "auth" {
		isAnon = true
	} else if authMsg.Token != "" {
		claims, err := h.verifier.Verify(authMsg.Token)
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

	validator := session.NewValidaator()
	sess := session.NewSession(conn, userID, isAnon, validator)
	h.sessions[sess.ID] = sess

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
		text, err := h.apiClient.GetText(context.Background(), textID)
		if err != nil {
			log.Printf("failed to get text %s: %v", textID, err)
			return
		}
		sess.StartSession(textID, text.Content)
		response := map[string]interface{}{"type": "session.started", "text_length": len(sess.TextRunes)}
		data, _ := json.Marshal(response)
		sess.Send <- data

	case "keystroke":
		char, ok := msg["char"].(string)
		if !ok {
			return
		}
		timestampF, ok := msg["timestamp"].(float64)
		if !ok {
			return
		}
		timestamp := int64(timestampF)

		res := sess.Validator.ValidateKS(sess, char, timestamp)
		if !res.IsValid {
			log.Printf("invalid keystroke: %s", res.Reason)
			return
		}

		progress, err := sess.ProcessKeystroke(char, timestamp)
		if err != nil {
			log.Printf("process keystroke error: %v", err)
			return
		}
		data, _ := json.Marshal(progress)
		sess.Send <- data

	case "session.finish":
		payload, run := sess.FinishSession()
		data, _ := json.Marshal(payload)
		sess.Send <- data

		go func(r *apiclient.RunResult, s *session.Session) {
			if h.apiClient != nil {
				if err := h.apiClient.SubmitRun(context.Background(), r); err != nil {
					log.Printf("failed to submit run: %v", err)
				}
			}
			s.Close()
			delete(h.sessions, s.ID)
		}(run, sess)

	default:
		log.Printf("unknown message type: %s", msgType)
	}
}
