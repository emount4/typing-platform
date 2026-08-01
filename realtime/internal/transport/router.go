package transport

import (
	"github.com/emount4/typing-realtime/internal/transport/handlers"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	router := gin.Default()

	api := router.Group("/api")
	{
		api.GET("/health", handlers.HealthHandler)
	}

	return router
}
