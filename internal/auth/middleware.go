package auth

import (
	"errors"
	"log"
	"net/http"

	"issue-tracker/internal/database/sqlc"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const currentUserKey = "currentUser"

// RequireSession authenticates requests by the session_token cookie. On success
// the user is stored on the context (see CurrentUser); otherwise the request is
// aborted with 401 (or 500 for unexpected database errors).
func RequireSession(queries *sqlc.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		ctx := c.Request.Context()
		session, err := queries.GetActiveSessionByTokenHash(ctx, HashSessionToken(cookie.Value))
		if errors.Is(err, pgx.ErrNoRows) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired or invalid"})
			return
		}
		if err != nil {
			log.Printf("session lookup failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		user, err := queries.GetUserByID(ctx, session.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired or invalid"})
			return
		}
		if err != nil {
			log.Printf("user lookup for session failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.Set(currentUserKey, &user)
		c.Next()
	}
}

// CurrentUser returns the user authenticated by RequireSession. It returns
// false if the middleware did not run for this request.
func CurrentUser(c *gin.Context) (*sqlc.User, bool) {
	v, ok := c.Get(currentUserKey)
	if !ok {
		return nil, false
	}
	user, ok := v.(*sqlc.User)
	return user, ok && user != nil
}
