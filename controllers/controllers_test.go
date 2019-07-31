package controllers

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"unsafe"

	"github.com/frullah/gin-boilerplate/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/frullah/gin-boilerplate/db"
	"golang.org/x/crypto/bcrypt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jsoniter "github.com/json-iterator/go"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

type dbMockMap = map[db.Instance][]sqlExpect

type tSetup struct {
	LoadRoutes bool
}

type tRequest struct {
	Method string
	URL    string
	data   tRequestData
}

type tRequestData struct {
	Header http.Header
	Query  url.Values
	Body   interface{}
}

type routeTestCase struct {
	name             string
	URL              string
	Method           string
	ExpectedBody     string
	ExpectHasHeaders []string
	Params           gin.Params
	Query            url.Values
	Header           http.Header
	Body             interface{}
	DB               dbMockMap
	ExpectedCode     int
}

type sqlExpect struct {
	expectedSQL []string
	result      interface{}
	transaction bool
}

// Test ...
type Test struct {
	router *gin.Engine
	db     *gorm.DB
}

var testUserData models.User
var testDisabledUserData models.User

func init() {
	bcryptCost = bcrypt.MinCost

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	gin.DefaultErrorWriter = os.Stderr
	gin.SetMode(gin.TestMode)

	testUserData = models.User{
		ID:       uint64(1),
		Username: "tester",
		Email:    "tester@domain.tld",
		Name:     "Tester",
		Role: &models.UserRole{
			ID:   uint64(1),
			Name: "",
		},
	}

	testDisabledUserData = models.User{}
}

func (c *routeTestCase) run(t *testing.T, engine *gin.Engine) {
	t.Helper()
	testRoute(t, engine, c)
}

func (c *Test) Serve(params tRequest) *httptest.ResponseRecorder {
	request := createRequest(params)
	response := httptest.NewRecorder()
	c.router.ServeHTTP(response, request)
	return response
}

func (c *Test) Teardown() {
	c.db.Close()
}

func (c routeTestCase) initDB(instance db.Instance, expects ...sqlExpect) {
	if c.DB == nil {
		c.DB = dbMockMap{}
	}
	c.DB[instance] = expects
}

// Setup - e2e testing setup
func Setup(params tSetup) *Test {
	router := gin.New()

	if params.LoadRoutes {
		LoadRoutes(router)
	}

	return &Test{
		router: router,
	}
}

func TestControllers(t *testing.T) {
	_ = FieldErrors{&FieldError{}}.Error()
}

func TestErrorMiddleware(t *testing.T) {
	dummyError := errors.New("testing ErrorMiddleware")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	t.Run("run after handler", func(t *testing.T) {
		const url = "/a-route-url"
		request, _ := http.NewRequest("GET", url, nil)
		router := gin.New()
		router.Use(ErrorMiddleware)
		router.GET(url, func(ctx *gin.Context) {
			ctx.Error(dummyError).SetType(gin.ErrorTypePrivate)
		})
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusInternalServerError, recorder.Code, "ErrorMiddleware is not running after handler")
	})

	t.Run("without error", func(t *testing.T) {
		ErrorMiddleware(context)
	})

	t.Run("unhandled bind", func(t *testing.T) {
		context.Error(dummyError).SetType(gin.ErrorTypeBind)
		ErrorMiddleware(context)
	})

	t.Run("unhandled private error type", func(t *testing.T) {
		context.Error(dummyError).SetType(gin.ErrorTypePrivate)
		ErrorMiddleware(context)
	})

	t.Run("unknown error type", func(t *testing.T) {
		context.Error(dummyError).SetType(gin.ErrorTypeAny)
		ErrorMiddleware(context)
	})

}

func toHTTPBody(body interface{}) io.Reader {
	if body == nil {
		return nil
	}

	switch x := body.(type) {
	case io.Reader:
		return x
	default:
		marshalledBody, _ := jsoniter.Marshal(x)
		return bytes.NewBuffer(marshalledBody)
	}
}

func createRequest(params tRequest) *http.Request {
	request, _ := http.NewRequest(
		params.Method,
		params.URL,
		toHTTPBody(params.data.Body),
	)

	if params.data.Header != nil {
		request.Header = params.data.Header
	}
	if params.data.Query != nil {
		request.URL.RawQuery = params.data.Query.Encode()
	}

	return request
}

func runHandlers(ctx *gin.Context, handlers []gin.HandlerFunc) {
	for _, handler := range handlers {
		handler(ctx)
		if ctx.IsAborted() {
			break
		}
	}
}

func testRoute(t *testing.T, router *gin.Engine, testCase *routeTestCase) {
	t.Helper()

	requestParams := tRequest{
		URL: testCase.URL,
		data: tRequestData{
			Query:  testCase.Query,
			Body:   testCase.Body,
			Header: testCase.Header,
		},
	}
	if testCase.Method != "" {
		requestParams.Method = testCase.Method
	} else {
		requestParams.Method = http.MethodGet
	}

	response := httptest.NewRecorder()
	req := createRequest(requestParams)

	if testCase.DB != nil {
		for instance, expects := range testCase.DB {
			sqlMock, teardown := db.SetupTest(instance)
			defer teardown()
			sqlmockExpects(sqlMock, expects...)
		}
	}
	router.ServeHTTP(response, req)

	if testCase.ExpectHasHeaders != nil {
		responseHeaders := response.Header()
		for _, key := range testCase.ExpectHasHeaders {
			if responseHeaders.Get(key) == "" {
				t.Errorf(`header "%s" expect to be have a value`, key)
			}
		}
	}
	if testCase.ExpectedCode > 0 {
		assert.Equal(t, testCase.ExpectedCode, response.Code)
	}
	if testCase.ExpectedBody != "" {
		responseBodyString := *(*string)(unsafe.Pointer(response.Body))
		if responseBodyString != "" {
			testJSONString(t, testCase.ExpectedBody, responseBodyString)
		}
	}
}

func sqlmockExpects(sqlMock sqlmock.Sqlmock, params ...sqlExpect) {
	for _, param := range params {
		sqlmockExpect(sqlMock, param)
	}
}

func sqlmockExpect(sqlMock sqlmock.Sqlmock, param sqlExpect) {
	if param.transaction {
		sqlMock.ExpectBegin()
	}
	for _, expectedSQL := range param.expectedSQL {
		if expectedSQL[:len("SELECT ")] == "SELECT " {
			query := sqlMock.ExpectQuery(expectedSQL)
			switch res := param.result.(type) {
			case *sqlmock.Rows:
				query.WillReturnRows(res)
			case error:
				query.WillReturnError(res)
				if param.transaction {
					sqlMock.ExpectRollback()
					return
				}
			default:
				panic(errors.New("result must be a *sqlmock.Rows or error"))
			}
		} else {
			exec := sqlMock.ExpectExec(expectedSQL)
			switch res := param.result.(type) {
			case driver.Result:
				exec.WillReturnResult(res)
			case error:
				exec.WillReturnError(res)
				if param.transaction {
					sqlMock.ExpectRollback()
					return
				}
			default:
				panic(errors.New("result must be a driver.Result or error"))
			}
		}
	}
	if param.transaction {
		sqlMock.ExpectCommit()
	}
}

func testJSONString(t *testing.T, expected, actual string) {
	t.Helper()

	expectedBuf := bytes.NewBufferString("")
	require.NoError(t, json.Compact(expectedBuf, []byte(expected)))

	actualBuf := bytes.NewBufferString("")
	require.NoError(t, json.Compact(actualBuf, []byte(actual)))

	assert.Equal(t,
		*(*string)(unsafe.Pointer(expectedBuf)),
		*(*string)(unsafe.Pointer(actualBuf)),
	)
}

func SetupRouter() *gin.Engine {
	router := gin.New()
	LoadRoutes(router)

	return router
}
