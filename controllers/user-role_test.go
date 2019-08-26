package controllers

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/frullah/gin-boilerplate/db"
	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

func TestUserRoleGetOne(t *testing.T) {
	const url = "/user-roles"

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "invalid id param",
			url:          url + "/x",
			expectedCode: http.StatusBadRequest,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "role id is not found",
			url:          url + "/1",
			expectedCode: http.StatusNotFound,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{
						"SELECT .+ FROM .user_role.",
						gorm.ErrRecordNotFound,
						false,
					},
				},
			},
		},
		{
			name: "role id is found",
			url:  url + "/1",
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
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

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) { testCase.run(t, router) })
	}
}
func TestUserRoleGetMany(t *testing.T) {
	const url = "/user-roles"

	router := SetupRouter()
	cases := []routeTestCase{
		// internal error cases
		{
			name:         "handle find error",
			url:          url,
			expectedCode: http.StatusInternalServerError,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{"SELECT .+ FROM .user_role.", errDummy, false},
				},
			},
		},
		{
			name:         "handle count error",
			url:          url,
			expectedCode: http.StatusInternalServerError,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{"SELECT .+ FROM .user_role.", sqlmock.NewRows([]string{}), false},
					{"SELECT count.+ FROM .user_role.", errDummy, false},
				},
			},
		},
		// success cases
		{
			name:         "empty data",
			url:          url,
			expectedCode: http.StatusOK,
			expectedBody: `{
				"status": "success",
				"data": {
					"count": 0,
					"items": []
				}
			}`,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{"SELECT .+ FROM .user_role.", sqlmock.NewRows([]string{}), false},
					{
						"SELECT count.+ FROM .user_role.",
						sqlmock.NewRows([]string{"count(*)"}).AddRow(0),
						false,
					},
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) { testCase.run(t, router) })
	}
}
func TestUserRoleCreateOne(t *testing.T) {
	const url = "/user-roles"
	const method = http.MethodPost

	router := SetupRouter()
	cases := []routeTestCase{
		// client error cases
		routeTestCase{
			name:         "empty body",
			url:          url,
			method:       method,
			expectedCode: http.StatusBadRequest,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "name exists",
			expectedCode: http.StatusConflict,
			url:          url,
			method:       method,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{
						`INSERT INTO .user_role.`,
						&mysql.MySQLError{Number: uint16(1062)},
						true,
					},
				},
			},
			body: `{"name": "exists-user-role"}`,
		},
		// success cases
		{
			name:   "valid body",
			url:    url,
			method: method,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			expectedCode: http.StatusOK,
			expectedBody: `{
				"status": "success",
				"data": {
					"id": 1
				}
			}`,
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{`INSERT INTO .user_role.`, sqlmock.NewResult(1, 1), true},
				},
			},
			body: `{"name": "new-user-role", "enabled": true}`,
		},
	}

	for _, handler := range cases {
		t.Run(handler.name, func(t *testing.T) { handler.run(t, router) })
	}
}

func TestUserRoleUpdate(t *testing.T) {
	const url = "/user-roles"
	const method = http.MethodPut

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "invalid id param",
			url:          url + "/x",
			method:       method,
			expectedCode: http.StatusBadRequest,
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
				},
			},
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
		},
		{
			name:         "id not found",
			url:          url + "/1",
			method:       method,
			expectedCode: http.StatusNotFound,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{
						"UPDATE .user_role. SET",
						gorm.ErrRecordNotFound,
						true,
					},
				},
			},
		},
		{
			name:         "id found",
			url:          url + "/1",
			method:       method,
			expectedCode: http.StatusOK,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			body: `{"name": "new user role name"}`,
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{
						"UPDATE .user_role. SET",
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

func TestUserRoleDelete(t *testing.T) {
	const url = "/user-roles"
	const method = http.MethodDelete

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:         "invalid id param",
			url:          url + "/x",
			method:       method,
			expectedCode: http.StatusBadRequest,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
				},
			},
		},
		{
			name:         "id not found",
			url:          url + "/1",
			method:       method,
			expectedCode: http.StatusNotFound,
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{
						"DELETE FROM .user_role.",
						gorm.ErrRecordNotFound,
						true,
					},
				},
			},
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
		},
		{
			name:         "id found",
			url:          url + "/1",
			method:       method,
			expectedCode: http.StatusOK,
			header: http.Header{
				AccessTokenHeader: []string{makeAccessToken(1)},
			},
			db: dbMockMap{
				db.Default: {
					sqlExpectAuthRole("administrator"),
					{
						"DELETE FROM .user_role.",
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
