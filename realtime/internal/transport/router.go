package transport

import "github.com/gin-gonic/gin"

func SetupRouter(verifier Verifier) *gin.Engine {
	router := gin.Default()

	healthHandler := NewHealthHandler()
	wsHandler := NewWebSocketHandler(verifier)

	api := router.Group("/api")
	{
		api.GET("/realtime_health", healthHandler.Health)
	}

	router.GET("/ws", wsHandler.HandleWS)

	ws := router.Group("/ws")
	{
		ws.GET("/echo", wsHandler.Echo)
	}
	return router
}
