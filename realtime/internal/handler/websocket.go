package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // В продакшене настройте строже
	},
}

type WebSocketHandler struct{}

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{}
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
