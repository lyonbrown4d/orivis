package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/arcgolabs/authx"
	authjwt "github.com/arcgolabs/authx/jwt"
	"github.com/golang-jwt/jwt/v5"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

const (
	dashboardAuthCookie = "orivis_dashboard_token"
	dashboardTokenTTL   = 12 * time.Hour
)

type dashboardJWTClaims struct {
	Subject string
}

func (e *dashboardEndpoint) createDashboardJWT(username string) (string, error) {
	secret, err := e.dashboardJWTSecret()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := authjwt.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strings.TrimSpace(username),
			Issuer:    "orivis",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(dashboardTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign dashboard JWT: %w", err)
	}
	return signed, nil
}

func (e *dashboardEndpoint) authenticateDashboardJWT(ctx context.Context, token string) bool {
	if !e.cfg.Auth.Dashboard.Enabled {
		return true
	}
	secret, err := e.dashboardJWTSecret()
	if err != nil {
		return false
	}
	claims, err := verifyDashboardJWT(ctx, token, []byte(secret))
	if err != nil {
		return false
	}
	return strings.TrimSpace(claims.Subject) != ""
}

func (e *dashboardEndpoint) dashboardJWTSetCookie(token string, expired bool) string {
	maxAge := int(dashboardTokenTTL.Seconds())
	if expired {
		maxAge = 0
	}
	cookie := dashboardAuthCookie + "=" + token + "; Path=" + dashboardCookiePath(e.cfg) + "; HttpOnly; SameSite=Lax; Max-Age=" + strconv.Itoa(maxAge)
	if e.cfg.Auth.Dashboard.SecureCookie {
		cookie += "; Secure"
	}
	return cookie
}

func dashboardCookiePath(cfg config.Config) string {
	if basePath := httpBasePath(cfg); basePath != "" {
		return basePath
	}
	return "/"
}

func (e *dashboardEndpoint) dashboardJWTSecret() (string, error) {
	secret := strings.TrimSpace(e.cfg.Auth.Dashboard.JWTSecret)
	if secret == "" {
		secret = e.cfg.Auth.Dashboard.Password
	}
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("dashboard JWT secret is not configured")
	}
	return secret, nil
}

func verifyDashboardJWT(ctx context.Context, token string, secret []byte) (dashboardJWTClaims, error) {
	provider := authjwt.NewProvider(
		authjwt.WithIssuer("orivis"),
		authjwt.WithHMACSecret(secret, jwt.SigningMethodHS256.Alg()),
		authjwt.WithRequiredExpiration(),
		authjwt.WithRequiredIssuedAt(),
	)
	result, err := provider.Authenticate(ctx, authjwt.NewTokenCredential(token))
	if err != nil {
		return dashboardJWTClaims{}, fmt.Errorf("authenticate dashboard JWT: %w", err)
	}
	principal, ok := authx.PrincipalFromAny(result.Principal)
	if !ok {
		return dashboardJWTClaims{}, errors.New("dashboard JWT principal is invalid")
	}
	if strings.TrimSpace(principal.ID) == "" {
		return dashboardJWTClaims{}, errors.New("dashboard JWT subject is empty")
	}
	return dashboardJWTClaims{Subject: principal.ID}, nil
}
