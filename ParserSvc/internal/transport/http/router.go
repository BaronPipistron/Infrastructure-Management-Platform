package httpapi

import (
	"net/http"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gin-gonic/gin"
)

func NewRouter(handler *Handler) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz)

	v1 := router.Group("/api/v1")
	{
		v1.GET("/desired-state", handler.GetDesiredState)
		v1.GET("/desired-state/hosts", handler.ListHosts)
		v1.GET("/desired-state/hosts/:hostId", handler.GetHostByID)
	}

	swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler)
	router.GET("/swagger/*any", func(ctx *gin.Context) {
		any := ctx.Param("any")
		if any == "" || any == "/" {
			ctx.Redirect(http.StatusFound, "/swagger/index.html")
			return
		}
		swaggerHandler(ctx)
	})

	return router
}
