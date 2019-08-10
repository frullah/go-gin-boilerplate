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

func init() {
	bcryptCost = bcrypt.MinCost
}

func TestAuthLogin(t *testing.T) {
	const url = "/auth/login"
	const method = http.MethodPost
	makeBody := func(username, password string) string {
		return `{"username": "` + username + `", "password": "` + password + `"}`
	}
	password := "secret"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)

	router := SetupRouter()
	cases := []routeTestCase{
		// error handling cases
		{
			name:         "handle db error",
			url:          url,
			method:       method,
			expectedCode: http.StatusInternalServerError,
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						errors.New("test: Login controller error handling"),
						false,
					},
				},
			},
			body: makeBody("username", password),
		},
		// client error cases
		{
			name:         "invalid body format",
			url:          url,
			method:       method,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"password": "password must be at least 5 characters in length",
					"username": "username must be at least 5 characters in length"
				}
			}`,
			body: makeBody("x", "x"),
		},
		{
			name:         "user not found",
			url:          url,
			method:       method,
			expectedCode: http.StatusUnauthorized,
			db: dbMockMap{
				db.Default: []sqlExpect{
					{[]string{"SELECT .+ FROM .user."}, gorm.ErrRecordNotFound, false},
				},
			},
			body: makeBody("unknown-username", password),
		},
		{
			name:         "disabled user",
			url:          url,
			method:       method,
			expectedCode: http.StatusForbidden,
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"enabled"}).AddRow(false),
						false,
					},
				},
			},
			body: makeBody("username", password),
		},
		{
			name:         "invalid password",
			url:          url,
			method:       method,
			expectedCode: http.StatusUnauthorized,
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"password", "enabled"}).
							AddRow("hashed-password", true),
						false,
					},
				},
			},
			body: makeBody("username", "invalid-password"),
		},
		// success cases
		{
			name:         "valid body format",
			url:          url,
			method:       method,
			expectedCode: http.StatusOK,
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"id", "password", "enabled"}).
							AddRow(uint64(1), hashedPassword, true),
						false,
					},
				},
			},
			body: makeBody("username", password),
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
	router.GET("/", AuthRolesMiddleware(nil))
	routeCases := []routeTestCase{
		{
			name:         "without access token",
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "invalid access token",
			expectedCode: http.StatusUnauthorized,
			header:       http.Header{accessTokenHeader: []string{"invalid token"}},
		},
		{
			name:         "invalid token method",
			expectedCode: http.StatusUnauthorized,
			header:       http.Header{accessTokenHeader: []string{unacceptedToken}},
		},
		{
			name:         "expired access token",
			expectedCode: http.StatusUnauthorized,
			header: http.Header{
				accessTokenHeader: []string{expiredAccessToken},
			},
		},
		{
			name:         "expired access token with refresh token but different user id",
			expectedCode: http.StatusUnauthorized,
			header: http.Header{
				accessTokenHeader:  []string{expiredAccessToken},
				refreshTokenHeader: []string{makeRefreshToken(3, "")},
			},
		},
		{
			name:         "expired access token with refresh token but user id not found",
			expectedCode: http.StatusUnauthorized,
			header: http.Header{
				accessTokenHeader:  []string{expiredAccessToken},
				refreshTokenHeader: []string{validRefreshToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						gorm.ErrRecordNotFound,
						false,
					},
				},
			},
		},
		{
			name:         "expired access token with refresh token but disabled user",
			expectedCode: http.StatusUnauthorized,
			header: http.Header{
				accessTokenHeader:  []string{expiredAccessToken},
				refreshTokenHeader: []string{validRefreshToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"enabled", "password"}).
							AddRow(false, "user-password-hash"),
						false,
					},
					{
						[]string{"SELECT .+"},
						sqlmock.NewRows([]string{"name"}).
							AddRow("administrator"),
						false,
					},
				},
			},
		},
		{
			name:             "expired access token with valid refresh token",
			expectedCode:     http.StatusUnauthorized,
			expectHasHeaders: []string{accessTokenHeader, refreshTokenHeader},
			header: http.Header{
				accessTokenHeader:  []string{expiredAccessToken},
				refreshTokenHeader: []string{validRefreshToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"enabled", "password", "role_id"}).
							AddRow(true, "user-password-hash", uint64(1)),
						false,
					},
					{
						[]string{"SELECT .+"},
						sqlmock.NewRows([]string{"name"}).
							AddRow("administrator"),
						false,
					},
				},
			},
		},
		routeTestCase{
			name:         "valid access token",
			expectedCode: http.StatusOK,
			header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+"},
						sqlmock.NewRows([]string{"name"}).
							AddRow("administrator"),
						false,
					},
				},
			},
		},
	}

	for _, routeCase := range routeCases {
		t.Run(routeCase.name, func(t *testing.T) { routeCase.run(t, router) })
	}
}
