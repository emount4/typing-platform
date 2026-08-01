package transport

import (
	"github.com/emount4/typing-realtime/internal/handler"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	router := gin.Default()

	healthHandler := handler.NewHealthHandler()
	wsHandler := handler.NewWebSocketHandler()

	api := router.Group("/api")
	{
		api.GET("/health", healthHandler.Health)
	}

	ws := router.Group("/ws")
	{
		ws.GET("/ws/", wsHandler.Echo)
	}
	return router
}
