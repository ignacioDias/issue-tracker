package auth

import (
	"context"
	"fmt"
	"issue-tracker/internal/database/sqlc"
	"issue-tracker/internal/user"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleOauth2Service interface {
	GetGoogleOAuth2LoginURL(state, verifier string) string
	GoogleOAuth2Callback(ctx context.Context, code, state, expectedState, verifier string) (*user.User, error)
	CreateSession(ctx context.Context, userID pgtype.UUID) (string, *Session, error)
}

type googleOAuth2Service struct {
	queries           *sqlc.Queries
	googleOAuthConfig *oauth2.Config
}

const expiringTimeSessionDuration = time.Hour

func NewGoogleOauth2Service(queries *sqlc.Queries, googleOAuthCallbackPath string) GoogleOauth2Service {
	return &googleOAuth2Service{
		queries: queries,
		googleOAuthConfig: &oauth2.Config{
			RedirectURL:  googleOAuthRedirectURL(googleOAuthCallbackPath),
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			Scopes: []string{ // Permissions we're asking for
				"openid",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint, // Google's OAuth2 endpoints
		},
	}
}

func googleOAuthRedirectURL(googleOAuthCallbackPath string) string {
	if redirectURL := os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"); redirectURL != "" {
		return redirectURL
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return os.Getenv("HOST") + ":" + port + googleOAuthCallbackPath
}

func (s *googleOAuth2Service) GetGoogleOAuth2LoginURL(state, verifier string) string {
	return ""
}

func (s *googleOAuth2Service) GoogleOAuth2Callback(ctx context.Context, code, state, expectedState, verifier string) (*user.User, error) {
	return nil, nil
}

func (s *googleOAuth2Service) CreateSession(ctx context.Context, userID pgtype.UUID) (string, *Session, error) {
	token, tokenHash, err := GenerateSessionToken()
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}
	arg := sqlc.CreateSessionParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(expiringTimeSessionDuration), Valid: true},
	}
	row, err := s.queries.CreateSession(ctx, arg)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}
	sessionModel, err := sessionFromRow(row)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}
	return token, sessionModel, nil
}
