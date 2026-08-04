package transport

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/emount4/typing-realtime/internal/auth"
	"github.com/emount4/typing-realtime/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // В продакшене настройте строже
	},
}

type Verifier interface {
	Verify(tokenString string) (*auth.JWTClaims, error)
}

type Sessions interface {
}

type WebSocketHandler struct {
	upgrader websocket.Upgrader
	verifier Verifier
	sessions map[string]*session.Session
}

func NewWebSocketHandler(verifier Verifier) *WebSocketHandler {
	return &WebSocketHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // В проде настроить под свой домен (Caddy)
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		verifier: verifier,
		sessions: make(map[string]*session.Session),
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

	// 1. Ждем auth-кадр с таймаутом 5 секунд (п. 4.3 доки)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Printf("auth timeout or error: %v", err)
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{}) // Сбрасываем дедлайн

	// 2. Парсим auth
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

	// 3. Создаем сессию и запускаем pumps
	sess := session.NewSession(conn, userID, isAnon)
	h.sessions[sess.ID] = sess

	log.Printf("session created: %s, user: %d, anon: %v", sess.ID, userID, isAnon)

	go sess.WritePump()
	go sess.ReadPump(h.messageHandler)
}

func (h *WebSocketHandler) messageHandler(sess *session.Session, msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		log.Printf("⚠️ Сообщение без type: %v", msg)
		return
	}

	log.Printf("📨 Получено сообщение type=%s от сессии %s", msgType, sess.ID)

	switch msgType {
	case "auth":
		// auth уже обработан при подключении, сюда не должно приходить
		log.Printf("⚠️ Повторный auth от сессии %s", sess.ID)

	case "session.start":
		log.Printf(" session.start: text_id=%v", msg["text_id"])
		// TODO: логика начала забега

	case "keystroke":
		log.Printf("⌨️ keystroke: char=%v, timestamp=%v", msg["char"], msg["timestamp"])
		// TODO: валидация и обработка нажатия

	case "session.finish":
		log.Printf(" session.finish")
		// TODO: подсчет результатов

	default:
		log.Printf(" Неизвестный тип сообщения: %s", msgType)
	}
}
