package router

import (
	"github.com/gin-gonic/gin"
	internalController "github.com/q1ngyang/rustdesk-api-kessoku/v3/http/controller/internalapi"
)

func InternalAuthInit(g *gin.Engine) {
	controller := &internalController.Auth{}
	group := g.Group("/api/internal/v1/auth")
	group.GET("/jwks", controller.JWKS)
	group.POST("/introspect", controller.Introspect)
}
