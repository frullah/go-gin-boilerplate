package controllers

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"unsafe"

	"github.com/frullah/gin-boilerplate/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/frullah/gin-boilerplate/db"

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
	header http.Header
	Body   interface{}
}

type routeTestCase struct {
	name             string
	url              string
	method           string
	expectedBody     string
	expectHasHeaders []string
	header           http.Header
	body             interface{}
	db               dbMockMap
	expectedCode     int
}

type sqlExpect struct {
	expectedSQL string
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
var errDummy = errors.New("testing")

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	gin.DefaultErrorWriter = os.Stderr
	gin.SetMode(gin.TestMode)
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
	if c.db == nil {
		c.db = dbMockMap{}
	}
	c.db[instance] = expects
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
	_ = FieldError{"field": "message"}.Error()
}

func TestErrorMiddleware(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	t.Run("run after handler", func(t *testing.T) {
		const url = "/a-route-url"
		request, _ := http.NewRequest("GET", url, nil)
		router := gin.New()
		router.Use(ErrorMiddleware)
		router.GET(url, func(ctx *gin.Context) {
			ctx.Error(errDummy).SetType(gin.ErrorTypePrivate)
		})
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusInternalServerError, recorder.Code, "ErrorMiddleware is not running after handler")
	})

	t.Run("without error", func(t *testing.T) {
		ErrorMiddleware(ctx)
	})

	t.Run("unhandled bind", func(t *testing.T) {
		ctx.Error(errDummy).SetType(gin.ErrorTypeBind)
		ErrorMiddleware(ctx)
	})

	t.Run("unhandled private error type", func(t *testing.T) {
		ctx.Error(errDummy).SetType(gin.ErrorTypePrivate)
		ErrorMiddleware(ctx)
	})

	t.Run("unknown error type", func(t *testing.T) {
		ctx.Error(errDummy).SetType(gin.ErrorTypeAny)
		ErrorMiddleware(ctx)
	})
}

func toHTTPBody(body interface{}) io.Reader {
	switch x := body.(type) {
	case nil:
		return nil
	case io.Reader:
		return x
	case string:
		return bytes.NewBufferString(x)
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

	if params.data.header != nil {
		request.Header = params.data.header
	}

	return request
}

func testRoute(t *testing.T, router *gin.Engine, testCase *routeTestCase) {
	t.Helper()

	requestParams := tRequest{
		URL: testCase.url,
		data: tRequestData{
			Body:   testCase.body,
			header: testCase.header,
		},
	}
	if testCase.method != "" {
		requestParams.Method = testCase.method
	} else {
		requestParams.Method = http.MethodGet
	}

	response := httptest.NewRecorder()
	req := createRequest(requestParams)

	if testCase.db != nil {
		for instance, expects := range testCase.db {
			sqlMock, teardown := db.SetupTest(instance)
			defer func() {
				if err := sqlMock.ExpectationsWereMet(); err != nil {
					t.Error(err)
				}
			}()
			defer teardown()
			sqlmockExpects(sqlMock, expects...)
		}
	}
	router.ServeHTTP(response, req)

	if testCase.expectHasHeaders != nil {
		responseHeaders := response.Header()
		for _, key := range testCase.expectHasHeaders {
			if responseHeaders.Get(key) == "" {
				t.Errorf(`header "%s" expect to be have a value`, key)
			}
		}
	}
	if testCase.expectedCode > 0 {
		assert.Equal(t, testCase.expectedCode, response.Code)
	}
	if testCase.expectedBody != "" {
		responseBodyString := *(*string)(unsafe.Pointer(response.Body))
		if responseBodyString != "" {
			assert.JSONEq(t, testCase.expectedBody, responseBodyString)
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

	if param.expectedSQL[:len("SELECT ")] == "SELECT " {
		query := sqlMock.ExpectQuery(param.expectedSQL)
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
		exec := sqlMock.ExpectExec(param.expectedSQL)
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

	if param.transaction {
		sqlMock.ExpectCommit()
	}
}

func SetupRouter() *gin.Engine {
	router := gin.New()
	LoadRoutes(router)

	return router
}

func sqlExpectAuthRole(roleName string) sqlExpect {
	return sqlExpect{
		expectedSQL: "SELECT .+ FROM .user.",
		result: sqlmock.NewRows([]string{"name"}).
			AddRow(roleName),
	}
}
