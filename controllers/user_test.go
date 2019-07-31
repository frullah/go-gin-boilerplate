package controllers

import (
	"net/http"
	"testing"

	"github.com/jinzhu/gorm"

	"github.com/frullah/gin-boilerplate/db"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUserGet(t *testing.T) {
	accessToken := makeAccessToken(1)

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "invalid id param",
			URL:          "/users/x",
			ExpectedCode: http.StatusBadRequest,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
		},
		{
			name:         "user id is not found",
			URL:          "/users/1",
			ExpectedCode: http.StatusNotFound,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			DB: dbMockMap{
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
			name:         "user id is found",
			URL:          "/users/1",
			ExpectedCode: http.StatusOK,
			ExpectedBody: `{
				"id": 1,
				"email": "user-email",
				"username": "user-username",
				"name": "user-name",
				"role": {
					"id": 1,
					"name": "user-role",
					"isEnabled": true
				},
				"isEnabled": true
			}`,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"id", "email",
							"username",
							"password",
							"name",
							"is_enabled",
							"role_id",
						}).
							AddRow(
								1,
								"user-email",
								"user-username",
								"user-password",
								"user-name",
								true,
								uint32(1),
							),
						false,
					},
					{
						[]string{"SELECT .+ FROM .user_role."},
						sqlmock.NewRows([]string{"id", "name", "is_enabled"}).
							AddRow(uint32(1), "user-role", true),
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

func TestUserRegister(t *testing.T) {
	accessToken := makeAccessToken(1)

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "empty body",
			URL:          "/register",
			Method:       http.MethodPost,
			ExpectedCode: http.StatusBadRequest,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
		},
		{
			name:         "valid body",
			URL:          "/register",
			Method:       http.MethodPost,
			ExpectedCode: http.StatusOK,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			Body: UserRegisterParams{
				Email:    "new-user@domain.tld",
				Username: "new-usr",
				Password: "new-user",
				Name:     "new-user",
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{[]string{"INSERT INTO .user."}, sqlmock.NewResult(1, 1), true},
				},
			},
		},
		{
			name:         "username or email is exists",
			ExpectedCode: http.StatusConflict,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			Body: UserRegisterParams{
				Email:    "new-user@domain.tld",
				Username: "new-usr",
				Password: "new-user",
				Name:     "new-user",
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{{
					[]string{"INSERT INTO .user."},
					&mysql.MySQLError{Number: uint16(1062)},
					true,
				}},
			},
			URL:    "/register",
			Method: http.MethodPost,
		},
	}

	for _, handler := range cases {
		t.Run(handler.name, func(t *testing.T) { handler.run(t, router) })
	}
}

func TestUserUpdate(t *testing.T) {
	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "invalid id param",
			URL:          "/users/x",
			Method:       http.MethodPut,
			ExpectedCode: http.StatusBadRequest,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
		},
		{
			name:         "id not found",
			URL:          "/users/1",
			Method:       http.MethodPut,
			ExpectedCode: http.StatusNotFound,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			Body: UserUpdateBody{
				Email: "email@domain.tld",
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"UPDATE .user. SET .+ WHERE"},
						gorm.ErrRecordNotFound,
						true,
					},
				},
			},
		},
	}

	for _, handler := range cases {
		t.Run(handler.name, func(t *testing.T) { handler.run(t, router) })
	}
}

func TestUserDelete(t *testing.T) {
	accessToken := makeAccessToken(1)

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "invalid id param",
			URL:          "/users/x",
			Method:       http.MethodDelete,
			ExpectedCode: http.StatusBadRequest,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
		},
		{
			name:         "id not found",
			URL:          "/users/1",
			Method:       http.MethodDelete,
			ExpectedCode: http.StatusNotFound,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"DELETE FROM .user. WHERE"},
						gorm.ErrRecordNotFound,
						true,
					},
				},
			},
		},
		{
			name:         "exists user id",
			URL:          "/users/2",
			Method:       http.MethodDelete,
			ExpectedCode: http.StatusOK,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"DELETE FROM .user. WHERE"},
						sqlmock.NewResult(1, 1),
						true,
					},
				},
			},
		},
	}

	for _, handler := range cases {
		t.Run(handler.name, func(t *testing.T) { handler.run(t, router) })
	}
}

func TestUserCreateOne(t *testing.T) {
	accessToken := makeAccessToken(1)

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "empty body",
			URL:          "/users",
			Method:       http.MethodPost,
			ExpectedCode: http.StatusBadRequest,
			ExpectedBody: jsonErrEmptyBody,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
		},
		{
			name:         "invalid body",
			URL:          "/users",
			Method:       http.MethodPost,
			ExpectedCode: http.StatusBadRequest,
			ExpectedBody: `[
				{"field": "email", "message": "email is a required field"},
				{"field": "username", "message": "username is a required field"},
				{"field": "password", "message": "password is a required field"},
				{"field": "name", "message": "name is a required field"},
				{"field": "roleId", "message": "roleId is a required field"}
			]`,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			Body: UserPostParams{},
		},
		{
			name:         "username or email is exists",
			URL:          "/users",
			Method:       http.MethodPost,
			ExpectedCode: http.StatusConflict,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{`INSERT INTO .user.`},
						&mysql.MySQLError{Number: uint16(1062)},
						true,
					},
				},
			},
			Body: UserPostParams{
				Email:    "new-user-email@domain.tld",
				Username: "new-user-username",
				Password: "new-user-password",
				Name:     "new-user",
				RoleID:   1,
			},
		},
		{
			name:         "valid body",
			URL:          "/users",
			Method:       http.MethodPost,
			ExpectedCode: http.StatusOK,
			ExpectedBody: `{"id": 1}`,
			Header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			DB: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{`INSERT INTO .user.`},
						sqlmock.NewResult(1, 1),
						true,
					},
				},
			},
			Body: UserPostParams{
				Email:    "new-user-email@domain.tld",
				Username: "new-user-username",
				Password: "new-user-password",
				Name:     "new-user",
				RoleID:   1,
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) { testCase.run(t, router) })
	}
}
