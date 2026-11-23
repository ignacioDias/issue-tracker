package router

import (
	"issue-tracker/internal/auth"

	"github.com/gin-gonic/gin"
)

type Router struct {
	engine              *gin.Engine
	googleOAuth2Handler auth.GoogleOAuth2Handler
}

func (r *Router) SetupRoutes() {

	v1 := r.engine.Group("/api/v1")

	v1.GET("/auth/oauth2/google", r.googleOAuth2Handler.GoogleOAuth2Login)
	v1.GET("/auth/oauth2/google/callback", r.googleOAuth2Handler.GoogleOAuth2Callback)

}
