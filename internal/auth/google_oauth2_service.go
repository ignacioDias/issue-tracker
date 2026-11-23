package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"issue-tracker/internal/database/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const googleProvider = "google"
const expiringTimeSessionDuration = time.Hour
const googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

var (
	ErrInvalidOAuthState      = errors.New("invalid oauth state")
	ErrGoogleEmailNotVerified = errors.New("google email is not verified")
	ErrEmailAccountExists     = errors.New("an account with this email already exists")
	ErrInvalidRedirectURL     = errors.New("invalid google oauth redirect url")
)

type GoogleOauth2Service interface {
	GetGoogleOAuth2LoginURL(state, verifier string) string
	GoogleOAuth2Callback(ctx context.Context, code, state, expectedState, verifier string) (*sqlc.User, error)
	CreateSession(ctx context.Context, userID pgtype.UUID) (string, *Session, error)
}

type googleOAuth2Service struct {
	db                *pgxpool.Pool
	queries           *sqlc.Queries
	googleOAuthConfig *oauth2.Config
}

func NewGoogleOauth2Service(db *pgxpool.Pool, googleOAuthCallbackPath string) (GoogleOauth2Service, error) {
	redirectURL, err := googleOAuthRedirectURL(googleOAuthCallbackPath)
	if err != nil {
		return nil, err
	}
	return &googleOAuth2Service{
		db:      db,
		queries: sqlc.New(db),
		googleOAuthConfig: &oauth2.Config{
			RedirectURL:  redirectURL,
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			Scopes: []string{ // Permissions we're asking for
				"openid",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint, // Google's OAuth2 endpoints
		},
	}, nil
}

// googleOAuthRedirectURL returns the absolute callback URL Google redirects to.
// GOOGLE_OAUTH_REDIRECT_URL takes precedence; otherwise it is built from HOST,
// PORT (default 8080) and the callback path. HOST must include the scheme,
// e.g. "http://localhost".
func googleOAuthRedirectURL(googleOAuthCallbackPath string) (string, error) {
	raw := os.Getenv("GOOGLE_OAUTH_REDIRECT_URL")
	if raw == "" {
		host := os.Getenv("HOST")
		if host == "" {
			return "", fmt.Errorf("%w: HOST or GOOGLE_OAUTH_REDIRECT_URL must be set", ErrInvalidRedirectURL)
		}
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		raw = host + ":" + port + googleOAuthCallbackPath
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidRedirectURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: scheme must be http or https, got %q", ErrInvalidRedirectURL, u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("%w: missing host in %q", ErrInvalidRedirectURL, raw)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%w: must not contain a query or fragment", ErrInvalidRedirectURL)
	}
	return raw, nil
}

func (s *googleOAuth2Service) GetGoogleOAuth2LoginURL(state, verifier string) string {
	return s.googleOAuthConfig.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// GoogleOAuth2Callback completes the Google login. It returns the existing user
// linked to this Google account, or creates one. Accounts are linked by email
// only when Google reports that email as verified.
func (s *googleOAuth2Service) GoogleOAuth2Callback(ctx context.Context, code, state, expectedState, verifier string) (*sqlc.User, error) {
	if expectedState == "" || state == "" || verifier == "" {
		return nil, ErrInvalidOAuthState
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
		return nil, ErrInvalidOAuthState
	}

	token, err := s.googleOAuthConfig.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange google code: %w", err)
	}

	info, err := fetchGoogleUserInfo(ctx, s.googleOAuthConfig.Client(ctx, token))
	if err != nil {
		return nil, err
	}
	if info.Sub == "" || info.Email == "" {
		return nil, errors.New("google profile is missing sub or email")
	}
	if !info.EmailVerified {
		return nil, ErrGoogleEmailNotVerified
	}

	email := strings.ToLower(strings.TrimSpace(info.Email))

	// If a concurrent first login for the same Google account wins the insert
	// race, our insert fails with a unique violation. The winner has committed
	// by then, so resolving again finds its account instead of returning 500.
	const maxAttempts = 2
	for attempt := 1; ; attempt++ {
		user, err := s.resolveOrCreateUser(ctx, info, email)
		if err == nil {
			return user, nil
		}
		if attempt >= maxAttempts || !isUniqueViolation(err) {
			return nil, err
		}
	}
}

// resolveOrCreateUser returns the account linked to info.Sub, or creates a new
// account and identity. It never links to an existing account by email alone.
func (s *googleOAuth2Service) resolveOrCreateUser(ctx context.Context, info *googleUserInfo, email string) (*sqlc.User, error) {
	identity, err := s.queries.GetOAuthIdentityByProviderSubject(ctx, sqlc.GetOAuthIdentityByProviderSubjectParams{
		Provider:        googleProvider,
		ProviderSubject: info.Sub,
	})
	if err == nil {
		user, err := s.queries.GetUserByID(ctx, identity.UserID)
		if err != nil {
			return nil, fmt.Errorf("get user for oauth identity: %w", err)
		}
		return &user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get oauth identity: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)

	// Never link to an existing account by email: the schema does not record
	// whether an account's email was verified, so we cannot prove it is safe.
	existing, err := q.GetUserByEmail(ctx, email)
	if err == nil {
		// The email may belong to this same Google account if a concurrent
		// login committed after our first identity lookup.
		linked, err := q.GetOAuthIdentityByProviderSubject(ctx, sqlc.GetOAuthIdentityByProviderSubjectParams{
			Provider:        googleProvider,
			ProviderSubject: info.Sub,
		})
		if err == nil && linked.UserID == existing.ID {
			return &existing, nil
		}
		return nil, ErrEmailAccountExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lookup user by email: %w", err)
	}

	displayName := strings.TrimSpace(info.Name)
	if displayName == "" {
		displayName = email
	}
	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Email:       email,
		DisplayName: displayName,
		AvatarUrl:   pgtype.Text{String: info.Picture, Valid: info.Picture != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	_, err = q.CreateOAuthIdentity(ctx, sqlc.CreateOAuthIdentityParams{
		UserID:          user.ID,
		Provider:        googleProvider,
		ProviderSubject: info.Sub,
		ProviderEmail:   pgtype.Text{String: email, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create oauth identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &user, nil
}

// isUniqueViolation reports whether err was caused by a unique constraint
// violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func fetchGoogleUserInfo(ctx context.Context, client *http.Client) (*googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build userinfo request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch google userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo returned status %d", resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode google userinfo: %w", err)
	}
	return &info, nil
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
