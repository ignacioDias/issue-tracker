package auth

import "github.com/gin-gonic/gin"

type GoogleOAuth2Handler interface {
	GoogleOAuth2Login(c *gin.Context)
	GoogleOAuth2Callback(c *gin.Context)
}
