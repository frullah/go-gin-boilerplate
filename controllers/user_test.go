package controllers

import (
	"errors"
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
		// client error cases
		{
			name:         "invalid id param",
			url:          "/users/x",
			expectedCode: http.StatusBadRequest,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "user id is not found",
			url:          "/users/1",
			expectedCode: http.StatusNotFound,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						[]string{"SELECT .+ FROM .user."},
						gorm.ErrRecordNotFound,
						false,
					},
				},
			},
		},
		// success cases
		{
			name:         "user id is found",
			url:          "/users/1",
			expectedCode: http.StatusOK,
			expectedBody: `{
				"id": 1,
				"email": "user-email",
				"username": "user-username",
				"name": "user-name",
				"enabled": true,
				"role": {
					"id": 1,
					"name": "user-role",
					"enabled": true
				}
			}`,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						[]string{"SELECT .+ FROM .user."},
						sqlmock.NewRows([]string{"id", "email",
							"username",
							"password",
							"name",
							"enabled",
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
						sqlmock.NewRows([]string{"id", "name", "enabled"}).
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

func TestUserAvailibility(t *testing.T) {
	router := SetupRouter()
	cases := []routeTestCase{
		// testing db error
		routeTestCase{
			name:         "handle db check error",
			url:          "/user-availibility?context=username&value=exists-username",
			expectedCode: http.StatusInternalServerError,
			db: dbMockMap{
				db.Default: []sqlExpect{{
					[]string{"SELECT .+ FROM .user. WHERE (username|email) = ?"},
					errors.New(""),
					false,
				}},
			},
		},
		// testing client error
		{
			name:         "empty query",
			url:          "/user-availibility",
			expectedCode: http.StatusBadRequest,
			expectedBody: jsonError("context must be an email or username"),
		},
		{
			name:         "invalid context",
			url:          "/user-availibility?context=neither-username-nor-email",
			expectedCode: http.StatusBadRequest,
			expectedBody: jsonError("context must be an email or username"),
		},
		routeTestCase{
			name:         "invalid username format",
			url:          "/user-availibility?context=username&value=x",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"value": "value must be at least 4 characters in length"
				}
			}`,
		},
		routeTestCase{
			name:         "unacceptable email",
			url:          "/user-availibility?context=email&value=unacceptable-email@domain",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"value": "value must be a valid email address"
				}
			}`,
		},
		// testing when success
		routeTestCase{
			name:         "username or email is not registered",
			url:          "/user-availibility?context=username&value=exists-username",
			expectedCode: http.StatusOK,
			expectedBody: `{
				"status": "success",
				"data": {
					"available": true
				}
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{{
					[]string{"SELECT .+ FROM .user. WHERE (username|email) = ?"},
					sqlmock.NewRows([]string{"1"}),
					false,
				}},
			},
		},
		routeTestCase{
			name:         "username or email is registered",
			url:          "/user-availibility?context=username&value=exists-username",
			expectedCode: http.StatusOK,
			expectedBody: `{
				"status": "success",
				"data": {
					"available": false
				}
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{{
					[]string{"SELECT .+ FROM .user. WHERE (username|email) = ?"},
					sqlmock.NewRows([]string{"1"}).
						AddRow("1"),
					false,
				}},
			},
		},
	}

	for _, handler := range cases {
		t.Run(handler.name, func(t *testing.T) { handler.run(t, router) })
	}
}

func TestUserRegister(t *testing.T) {
	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "empty body",
			url:          "/register",
			method:       http.MethodPost,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "valid body",
			url:          "/register",
			method:       http.MethodPost,
			expectedCode: http.StatusOK,
			body: `{
				"email": "new-user@domain.tld",
				"username": "new-usr",
				"password": "new-user",
				"name": "new-user"
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"INSERT INTO .user."},
						sqlmock.NewResult(1, 1),
						true,
					},
				},
			},
		},
		{
			name:         "username or email is exists",
			url:          "/register",
			method:       http.MethodPost,
			expectedCode: http.StatusConflict,
			body: `{
				"email": "new-user@domain.tld",
				"username": "new-usr",
				"password": "new-user",
				"name": "new-user"
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{
					{
						[]string{"INSERT INTO .user."},
						&mysql.MySQLError{Number: uint16(1062)},
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

func TestUserUpdate(t *testing.T) {
	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "invalid id param",
			url:          "/users/x",
			method:       http.MethodPut,
			expectedCode: http.StatusBadRequest,
			header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "id not found",
			url:          "/users/1",
			method:       http.MethodPut,
			expectedCode: http.StatusNotFound,
			header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			body: `{
				"email": "email@domain.tld",
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
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
			url:          "/users/x",
			method:       http.MethodDelete,
			expectedCode: http.StatusBadRequest,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "id not found",
			url:          "/users/1",
			method:       http.MethodDelete,
			expectedCode: http.StatusNotFound,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
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
			url:          "/users/2",
			method:       http.MethodDelete,
			expectedCode: http.StatusOK,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
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
		// client error cases
		{
			name:         "empty body",
			url:          "/users",
			method:       http.MethodPost,
			expectedCode: http.StatusBadRequest,
			expectedBody: jsonErrEmptyBody,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "invalid body",
			url:          "/users",
			method:       http.MethodPost,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"email": "email is a required field",
					"name": "name is a required field",
					"password": "password is a required field",
					"roleId": "roleId is a required field",
					"username": "username is a required field"
				}
			}`,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			body: `{}`,
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "username or email is exists",
			url:          "/users",
			method:       http.MethodPost,
			expectedCode: http.StatusConflict,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			body: `{
				"email": "new-user@domain.tld",
				"username": "new-usr",
				"password": "new-user",
				"name": "new-user",
				"roleId": 1
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						[]string{`INSERT INTO .user.`},
						&mysql.MySQLError{Number: uint16(1062)},
						true,
					},
				},
			},
		},
		// success cases
		{
			name:         "valid body",
			url:          "/users",
			method:       http.MethodPost,
			expectedCode: http.StatusOK,
			expectedBody: `{
				"status": "success",
				"data": {
					"id": 1
				}	
			}`,
			header: http.Header{
				accessTokenHeader: []string{accessToken},
			},
			body: `{
				"email": "new-user@domain.tld",
				"username": "new-usr",
				"password": "new-user",
				"name": "new-user",
				"roleId": 1
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						[]string{`INSERT INTO .user.`},
						sqlmock.NewResult(1, 1),
						true,
					},
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) { testCase.run(t, router) })
	}
}
