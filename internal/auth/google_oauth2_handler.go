package auth

import (
	"crypto/rand"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	oauthStateCookieName    = "oauthstate"
	oauthVerifierCookieName = "oauthcodeverifier"
	oauthCookieTTL          = 10 * time.Minute
	sessionCookieName       = "session_token"
)

type GoogleOAuth2Handler interface {
	GoogleOAuth2Login(c *gin.Context)
	GoogleOAuth2Callback(c *gin.Context)
}

type googleOAuth2Handler struct {
	service      GoogleOauth2Service
	secureCookie bool
}

func NewGoogleOAuth2Handler(service GoogleOauth2Service) GoogleOAuth2Handler {
	return &googleOAuth2Handler{
		service:      service,
		secureCookie: os.Getenv("APP_ENV") == "production",
	}
}

func (h *googleOAuth2Handler) GoogleOAuth2Login(c *gin.Context) {
	// The state prevents CSRF and the verifier is required for PKCE.
	state := rand.Text()
	verifier := oauth2.GenerateVerifier()
	h.setCookie(c, oauthStateCookieName, state, oauthCookieTTL)
	h.setCookie(c, oauthVerifierCookieName, verifier, oauthCookieTTL)

	c.Redirect(http.StatusFound, h.service.GetGoogleOAuth2LoginURL(state, verifier))
}

func (h *googleOAuth2Handler) GoogleOAuth2Callback(c *gin.Context) {
	stateCookie, stateErr := c.Request.Cookie(oauthStateCookieName)
	verifierCookie, verifierErr := c.Request.Cookie(oauthVerifierCookieName)
	// These are single-use values; clear them even if the callback fails.
	h.clearCookie(c, oauthStateCookieName)
	h.clearCookie(c, oauthVerifierCookieName)

	if stateErr != nil || verifierErr != nil {
		log.Printf("OAuth cookies not found for Google callback: state=%v verifier=%v", stateErr, verifierErr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "oauth cookies missing or expired"})
		return
	}
	if oauthError := c.Query("error"); oauthError != "" {
		log.Printf("Google OAuth authorization denied: %s", oauthError)
		c.JSON(http.StatusBadRequest, gin.H{"error": "google authorization was not granted"})
		return
	}

	ctx := c.Request.Context()
	user, err := h.service.GoogleOAuth2Callback(
		ctx,
		c.Query("code"),
		c.Query("state"),
		stateCookie.Value,
		verifierCookie.Value,
	)
	if err != nil {
		h.writeCallbackError(c, err)
		return
	}

	token, _, err := h.service.CreateSession(ctx, user.ID)
	if err != nil {
		log.Printf("Error creating session after google login: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error creating the session"})
		return
	}
	h.setCookie(c, sessionCookieName, token, expiringTimeSessionDuration)
	c.JSON(http.StatusOK, gin.H{"status": "logged in"})
}

// writeCallbackError maps service errors to HTTP responses. Unexpected errors
// are logged and reported generically so internal details don't leak.
func (h *googleOAuth2Handler) writeCallbackError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidOAuthState):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth state"})
	case errors.Is(err, ErrGoogleEmailNotVerified):
		c.JSON(http.StatusForbidden, gin.H{"error": "google email is not verified"})
	case errors.Is(err, ErrEmailAccountExists):
		c.JSON(http.StatusConflict, gin.H{"error": "an account with this email already exists"})
	default:
		log.Printf("Error while handling google callback: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error processing google oauth callback"})
	}
}

func (h *googleOAuth2Handler) setCookie(c *gin.Context, name, value string, ttl time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *googleOAuth2Handler) clearCookie(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}
