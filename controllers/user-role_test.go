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
			URL:          url + "/x",
			ExpectedCode: http.StatusBadRequest,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
		},
		{
			name:         "role id is not found",
			URL:          url + "/1",
			ExpectedCode: http.StatusNotFound,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			DB: dbMockMap{
				db.Default: {
					{
						[]string{"SELECT .+ FROM .user_role. WHERE"},
						gorm.ErrRecordNotFound,
						false,
					},
				},
			},
		},
		{
			name: "role id is found",
			URL:  url + "/1",
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			DB: dbMockMap{
				db.Default: {
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

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.run(t, router)
		})
	}
}
func TestUserRoleCreateOne(t *testing.T) {
	const url = "/user-roles"
	const method = http.MethodPost

	router := SetupRouter()
	cases := []routeTestCase{
		{
			name:   "empty body",
			URL:    url,
			Method: method,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			ExpectedCode: http.StatusBadRequest,
			ExpectedBody: jsonErrEmptyBody,
		},
		{
			name:         "name exists",
			ExpectedCode: http.StatusConflict,
			URL:          url,
			Method:       method,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			DB: dbMockMap{
				db.Default: {
					{
						[]string{`INSERT INTO .user_role.`},
						&mysql.MySQLError{Number: uint16(1062)},
						true,
					},
				},
			},
			Body: UserRoleCreateOneBody{Name: "exists-user-role"},
		},
		{
			name:   "valid body",
			URL:    url,
			Method: method,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			ExpectedCode: http.StatusOK,
			ExpectedBody: `{"id": 1}`,
			DB: dbMockMap{
				db.Default: {
					{
						[]string{`INSERT INTO .user_role.`},
						sqlmock.NewResult(1, 1),
						true,
					},
				},
			},
			Body: UserRoleCreateOneBody{
				Name:      "new-user-role",
				IsEnabled: true,
			},
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
			URL:          url + "/x",
			Method:       method,
			ExpectedCode: http.StatusBadRequest,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
		},
		{
			name:         "id not found",
			URL:          url + "/1",
			Method:       method,
			ExpectedCode: http.StatusNotFound,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			DB: dbMockMap{
				db.Default: {
					{
						[]string{"UPDATE .user_role. SET"},
						gorm.ErrRecordNotFound,
						true,
					},
				},
			},
		},
		{
			name:         "id found",
			URL:          url + "/1",
			Method:       method,
			ExpectedCode: http.StatusOK,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			Body: UserRoleUpdateBody{Name: "new user role name"},
			DB: dbMockMap{
				db.Default: {
					{
						[]string{"UPDATE .user_role. SET"},
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
			URL:          url + "/x",
			Method:       method,
			ExpectedCode: http.StatusBadRequest,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
		},
		{
			name:         "id not found",
			URL:          url + "/1",
			Method:       method,
			ExpectedCode: http.StatusNotFound,
			DB: dbMockMap{
				db.Default: {
					{[]string{"DELETE FROM .user_role."}, gorm.ErrRecordNotFound, true},
				},
			},
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
		},
		{
			name:         "id found",
			URL:          url + "/1",
			Method:       method,
			ExpectedCode: http.StatusOK,
			Header: http.Header{
				accessTokenHeader: []string{makeAccessToken(1)},
			},
			DB: dbMockMap{
				db.Default: {
					{[]string{"DELETE FROM .user_role."}, sqlmock.NewResult(1, 1), true},
				},
			},
		},
	}

	for _, handler := range cases {
		t.Run(handler.name, func(t *testing.T) { handler.run(t, router) })
	}
}
