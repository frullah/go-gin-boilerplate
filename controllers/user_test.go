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

func TestUserGetOne(t *testing.T) {
	accessToken := makeAccessToken(1)

	router := SetupRouter()
	cases := []routeTestCase{
		// client error cases
		{
			name:         "invalid id param",
			url:          "/users/x",
			expectedCode: http.StatusBadRequest,
			header: http.Header{
				AccessTokenHeader: []string{accessToken},
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
				AccessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						"SELECT .+ FROM .user.",
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
				"status": "success",
				"data": {
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
				}
			}`,
			header: http.Header{
				AccessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						"SELECT .+ FROM .user.",
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
						"SELECT .+ FROM .user_role.",
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
	const url = "/user-availibility"

	router := SetupRouter()
	cases := []routeTestCase{
		// internal error cases
		routeTestCase{
			name:         "handle db error",
			url:          url + "?context=username&value=exists-username",
			expectedCode: http.StatusInternalServerError,
			db: dbMockMap{
				db.Default: []sqlExpect{{
					"SELECT .+ FROM .user. WHERE username = ?",
					errDummy,
					false,
				}},
			},
		},
		// client error cases
		{
			name:         "empty query",
			url:          url,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"context": "context must be an email or username"
				}	
			}`,
		},
		{
			name:         "invalid context",
			url:          url + "?context=neither-username-nor-email",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"context": "context must be an email or username"
				}	
			}`,
		},
		routeTestCase{
			name:         "invalid username format",
			url:          url + "?context=username&value=x",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"value": "value must be at least 5 characters in length"
				}
			}`,
		},
		routeTestCase{
			name:         "invalid email format",
			url:          url + "?context=email&value=invalid-email.domain",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"value": "value must be a valid email address"
				}
			}`,
		},
		// success cases
		routeTestCase{
			name:         "username is not registered",
			url:          url + "?context=username&value=exists-username",
			expectedCode: http.StatusOK,
			expectedBody: `{
				"status": "success",
				"data": {
					"available": true
				}
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{{
					"SELECT .+ FROM .user. WHERE username = ?",
					sqlmock.NewRows([]string{"1"}),
					false,
				}},
			},
		},
		routeTestCase{
			name:         "username is registered",
			url:          url + "?context=username&value=exists-username",
			expectedCode: http.StatusOK,
			expectedBody: `{
				"status": "success",
				"data": {
					"available": false
				}
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{{
					"SELECT .+ FROM .user. WHERE username = ?",
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
	const url = "/register"

	router := SetupRouter()
	cases := []routeTestCase{
		// client error cases
		{
			name:         "empty body",
			url:          url,
			method:       http.MethodPost,
			expectedCode: http.StatusBadRequest,
		},
		routeTestCase{
			name:         "username too long",
			url:          url,
			method:       http.MethodPost,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{
				"status": "fail",
				"data": {
					"username": "username must be a maximum of 64 characters in length"		
				}
			}`,
			body: `{
				"email": "new-user@domain.tld",
				"username": "qwertyuiop1234567890123456789012345678901234567890123456789012345",
				"password": "new-user",
				"name": "new-user"
			}`,
		},
		{
			name:         "username or email is exists",
			url:          url,
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
						"INSERT INTO .user.",
						&mysql.MySQLError{Number: uint16(1062)},
						true,
					},
				},
			},
		},
		// success cases
		{
			name:         "valid body",
			url:          url,
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
						"INSERT INTO .user.",
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

func TestUserUpdate(t *testing.T) {
	router := SetupRouter()
	cases := []routeTestCase{
		// client error cases
		{
			name:         "invalid id param",
			url:          "/users/x",
			method:       http.MethodPut,
			expectedCode: http.StatusBadRequest,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "id not found",
			url:          "/users/0",
			method:       http.MethodPut,
			expectedCode: http.StatusNotFound,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			body: `{
				"email": "email@domain.tld",
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						"UPDATE .user. SET .+ WHERE",
						gorm.ErrRecordNotFound,
						true,
					},
				},
			},
		},
		// success cases
		{
			name:         "id found",
			url:          "/users/2",
			method:       http.MethodPut,
			expectedCode: http.StatusOK,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			body: `{
				"email": "email@domain.tld",
			}`,
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						"UPDATE .user. SET .+ WHERE",
						sqlmock.NewResult(0, 1),
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
				AccessTokenHeader: []string{accessToken},
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
				AccessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						"DELETE FROM .user. WHERE",
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
				AccessTokenHeader: []string{accessToken},
			},
			db: dbMockMap{
				db.Default: []sqlExpect{
					sqlExpectAuthRole("administrator"),
					{
						"DELETE FROM .user. WHERE",
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
			header: http.Header{
				AccessTokenHeader: []string{accessToken},
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
				AccessTokenHeader: []string{accessToken},
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
				AccessTokenHeader: []string{accessToken},
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
						`INSERT INTO .user.`,
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
				AccessTokenHeader: []string{accessToken},
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
						`INSERT INTO .user.`,
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
