package controllers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
						"SELECT .+ FROM .user.",
						errDummy,
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
					{"SELECT .+ FROM .user.", gorm.ErrRecordNotFound, false},
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
						"SELECT .+ FROM .user.",
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
						"SELECT .+ FROM .user.",
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
						"SELECT .+ FROM .user.",
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

func TestAuthData(t *testing.T) {
	const url = "/auth/data"
	createRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"enabled", "username", "name"})
	}

	router := SetupRouter()
	cases := []routeTestCase{
		// error handling cases
		routeTestCase{
			name:         "handle db error",
			url:          url,
			expectedCode: http.StatusInternalServerError,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						"SELECT .+ FROM .user.",
						errDummy,
						false,
					},
				},
			},
		},
		// client error cases
		routeTestCase{
			name:         "user not found",
			url:          url,
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{
				"status": "error",
				"message": "Unauthorized"
			}`,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{"SELECT .+ FROM .user.", gorm.ErrRecordNotFound, false},
				},
			},
		},
		{
			name:         "disabled user",
			url:          url,
			expectedCode: http.StatusForbidden,
			expectedBody: `{
				"status": "error",
				"message": "User disabled"
			}`,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						"SELECT .+ FROM .user.",
						createRows().
							AddRow(false, "member", "Member Name"),
						false,
					},
				},
			},
		},
		{
			name:         "disabled user role",
			url:          url,
			expectedCode: http.StatusForbidden,
			expectedBody: `{
				"status": "error",
				"message": "User disabled"
			}`,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						"SELECT .+ FROM .user.",
						createRows().
							AddRow(true, "member", "Member Name"),
						false,
					},
					{
						"SELECT .+ FROM .user_role.",
						sqlmock.NewRows([]string{"enabled", "name"}).
							AddRow(false, "member"),
						false,
					},
				},
			},
		},

		// success cases
		{
			name:         "success",
			url:          url,
			expectedCode: http.StatusOK,
			expectedBody: `{
				"status": "success",
				"data": {
					"username": "admin",
					"name": "Administrator",
					"role": "administrator"
				}
			}`,
			header: http.Header{
				AccessTokenHeader:  []string{makeAccessToken(1)},
				RefreshTokenHeader: []string{makeRefreshToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						"SELECT .+ FROM .user.",
						createRows().
							AddRow(true, "admin", "Administrator"),
						false,
					},
					{
						"SELECT .+ FROM .user_role.",
						sqlmock.NewRows([]string{"enabled", "name"}).
							AddRow(true, "administrator"),
						false,
					},
				},
			},
		},
	}

	for _, handler := range cases {
		t.Run(handler.name, func(t *testing.T) { handler.run(t, router) })
	}
}

func TestAuthMiddleware(t *testing.T) {
	var invalidToken string
	// create unaccepted method token
	{
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.StandardClaims{
			ExpiresAt: 0,
		})
		invalidToken, _ = token.SignedString(key)
	}
	validRefreshToken := makeRefreshToken(1)
	expiredAccessToken := makeJWT(1, -time.Second, accessTokenSecret)

	router := gin.New()
	router.GET("/", AuthRolesMiddleware(nil))
	router.GET("/roles", AuthRolesMiddleware(map[string]struct{}{
		"administrator": {},
	}))
	routeCases := []routeTestCase{
		// client error cases
		{
			name:         "without access token",
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{
				"status": "error",
				"message": "Unauthorized"	
			}`,
		},
		{
			name:         "invalid access token",
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{
				"status": "error",
				"message": "Unauthorized"	
			}`,
			header: http.Header{AccessTokenHeader: []string{"invalid token"}},
		},
		{
			name:         "invalid token method",
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{
				"status": "error",
				"message": "Unauthorized"	
			}`,
			header: http.Header{AccessTokenHeader: []string{invalidToken}},
		},
		{
			name:         "expired access token",
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{
				"status": "error",
				"message": "Unauthorized"	
			}`,
			header: http.Header{
				AccessTokenHeader: []string{expiredAccessToken},
			},
		},
		{
			name:         "expired access token with refresh token but different user id",
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{
				"status": "error",
				"message": "Unauthorized"	
			}`,
			header: http.Header{
				AccessTokenHeader:  []string{expiredAccessToken},
				RefreshTokenHeader: []string{makeRefreshToken(3)},
			},
		},
		{
			name:         "expired access token with valid refresh token but the user id is not found",
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{
				"status": "error",
				"message": "Unauthorized"	
			}`,
			header: http.Header{
				AccessTokenHeader:  []string{expiredAccessToken},
				RefreshTokenHeader: []string{validRefreshToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						"SELECT .+ FROM .user.",
						gorm.ErrRecordNotFound,
						false,
					},
				},
			},
		},
		{
			name:         "expired access token with valid refresh token but the user is disabled",
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{
				"status": "error",
				"message": "Unauthorized"	
			}`,
			header: http.Header{
				AccessTokenHeader:  []string{expiredAccessToken},
				RefreshTokenHeader: []string{validRefreshToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						"SELECT .+ FROM .user.",
						sqlmock.NewRows([]string{"enabled", "password"}).
							AddRow(false, "user-password-hash"),
						false,
					},
				},
			},
		},
		// success cases
		{
			name:             "expired access token with valid refresh token",
			expectedCode:     http.StatusOK,
			expectHasHeaders: []string{AccessTokenHeader, RefreshTokenHeader},
			header: http.Header{
				AccessTokenHeader:  []string{expiredAccessToken},
				RefreshTokenHeader: []string{validRefreshToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						"SELECT .+ FROM .user.",
						sqlmock.NewRows([]string{"enabled", "password", "role_id"}).
							AddRow(true, "user-password-hash", uint64(1)),
						false,
					},
				},
			},
		},
		routeTestCase{
			name:         "valid access token",
			expectedCode: http.StatusOK,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
		},
		routeTestCase{
			name:         "should prevent when role not included",
			url:          "/roles",
			expectedCode: http.StatusUnauthorized,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("user"),
				},
			},
		},
	}

	for _, routeCase := range routeCases {
		t.Run(routeCase.name, func(t *testing.T) { routeCase.run(t, router) })
	}
}
