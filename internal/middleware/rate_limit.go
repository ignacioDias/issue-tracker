package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func RateLimiter(rpm int, burst int) gin.HandlerFunc {
	limiter := rate.NewLimiter(
		rate.Every(time.Minute/time.Duration(rpm)),
		burst,
	)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}
