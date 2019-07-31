package controllers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/frullah/gin-boilerplate/db"
	"github.com/gin-gonic/gin"

	"golang.org/x/crypto/bcrypt"

	"github.com/jinzhu/gorm"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAuthLogin(t *testing.T) {
	const url = "/auth/login"
	const method = http.MethodPost

	password := "password"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "handle db error",
			URL:          url,
			Method:       method,
			ExpectedCode: http.StatusInternalServerError,
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						errors.New("test: auth.Login controller error handling"),
						false,
					},
				},
			},
			Body: AuthLoginParams{"username", "invalid-password"},
		},
		{
			name:         "invalid body format",
			URL:          url,
			Method:       method,
			ExpectedCode: http.StatusBadRequest,
			ExpectedBody: `[
				{"field": "username", "message": "username must be at least 5 characters in length"},
				{"field": "password", "message": "password must be at least 5 characters in length"}
				]`,
			Body: AuthLoginParams{"x", "x"},
		},
		{
			name:         "user not found",
			URL:          url,
			Method:       method,
			ExpectedCode: http.StatusUnauthorized,
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{[]string{"SELECT .+ FROM .user."}, gorm.ErrRecordNotFound, false},
				},
			},
			Body: AuthLoginParams{"username", "password"},
		},
		{
			name:         "invalid password",
			URL:          url,
			Method:       method,
			ExpectedCode: http.StatusUnauthorized,
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"password", "is_enabled"}).
							AddRow("hashed-password", true),
						false,
					},
				},
			},
			Body: AuthLoginParams{"username", "invalid-password"},
		},
		{
			name:         "valid body format",
			URL:          url,
			Method:       method,
			ExpectedCode: http.StatusOK,
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"id", "password", "is_enabled"}).
							AddRow(uint64(1), hashedPassword, true),
						false,
					},
				},
			},
			Body: AuthLoginParams{"username", password},
		},
		{
			name: "disabled user",
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"is_enabled"}).AddRow(false),
						false,
					},
				},
			},
			Body:         AuthLoginParams{"username", password},
			ExpectedCode: http.StatusForbidden,
			URL:          url,
			Method:       method,
		},
	}

	for _, handler := range cases {
		t.Run(handler.name, func(t *testing.T) { handler.run(t, router) })
	}
}
func TestAuthMiddleware(t *testing.T) {
	var unacceptedToken string
	// create unaccepted method token
	{
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.StandardClaims{
			ExpiresAt: 0,
		})
		unacceptedToken, _ = token.SignedString(key)
	}
	validRefreshToken := makeRefreshToken(1, "")
	expiredAccessToken := makeJWT(1, -time.Second, accessTokenSecret)

	router := gin.New()
	router.GET("/", AuthMiddleware)
	routeCases := []routeTestCase{
		{
			name:         "without access token",
			ExpectedCode: http.StatusUnauthorized,
		},
		{
			name:         "invalid access token",
			ExpectedCode: http.StatusUnauthorized,
			Header:       http.Header{accessTokenHeader: []string{"invalid token"}},
		},
		{
			name:         "invalid token method",
			ExpectedCode: http.StatusUnauthorized,
			Header:       http.Header{accessTokenHeader: []string{unacceptedToken}},
		},
		{
			name:         "expired access token",
			ExpectedCode: http.StatusUnauthorized,
			Header: http.Header{
				accessTokenHeader: []string{expiredAccessToken},
			},
		},
		{
			name:         "expired access token with refresh token but different user id",
			ExpectedCode: http.StatusUnauthorized,
			Header: http.Header{
				accessTokenHeader:  []string{expiredAccessToken},
				refreshTokenHeader: []string{makeRefreshToken(3, "")},
			},
		},
		{
			name:         "expired access token with refresh token but user id not found",
			ExpectedCode: http.StatusUnauthorized,
			Header: http.Header{
				accessTokenHeader:  []string{expiredAccessToken},
				refreshTokenHeader: []string{validRefreshToken},
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						gorm.ErrRecordNotFound,
						false,
					},
					{
						[]string{"SELECT .+ FROM .user_role."},
						sqlmock.NewRows([]string{"name"}).
							AddRow("administrator"),
						false,
					},
				},
			},
		},
		{
			name:         "expired access token with refresh token but disabled user",
			ExpectedCode: http.StatusUnauthorized,
			Header: http.Header{
				accessTokenHeader:  []string{expiredAccessToken},
				refreshTokenHeader: []string{validRefreshToken},
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"is_enabled", "password", "role_id"}).
							AddRow(false, "user-password-hash", uint64(1)),
						false,
					},
					{
						[]string{"SELECT .+ FROM .user_role."},
						sqlmock.NewRows([]string{"name"}).
							AddRow("administrator"),
						false,
					},
				},
			},
		},
		{
			name:             "expired access token with valid refresh token",
			ExpectedCode:     http.StatusUnauthorized,
			ExpectHasHeaders: []string{accessTokenHeader, refreshTokenHeader},
			Header: http.Header{
				accessTokenHeader:  []string{expiredAccessToken},
				refreshTokenHeader: []string{validRefreshToken},
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"is_enabled", "password", "role_id"}).
							AddRow(true, "user-password-hash", uint64(1)),
						false,
					},
					{
						[]string{"SELECT .+ FROM .user_role."},
						sqlmock.NewRows([]string{"name"}).
							AddRow("administrator"),
						false,
					},
				},
			},
		},
		{
			name: "valid access token",
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			ExpectedCode: http.StatusOK,
		},
	}

	for _, routeCase := range routeCases {
		t.Run(routeCase.name, func(t *testing.T) { routeCase.run(t, router) })
	}
}
