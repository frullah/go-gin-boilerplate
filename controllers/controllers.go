package controllers

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"

	ginvalidator "github.com/frullah/gin-validator"
	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/logrusorgru/aurora"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"

	"gopkg.in/go-playground/validator.v9"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	en_translations "gopkg.in/go-playground/validator.v9/translations/en"
)

// IntID - response of new data
type IntID struct {
	ID int `json:"id"`
}

// Uint64ID response of new big data
type Uint64ID struct {
	ID uint64 `json:"id"`
}

// FieldError object
type FieldError map[string]string

// MessageResponse ...
type MessageResponse struct {
	Message string `json:"message"`
}

const (
	userURL     = "/users"
	userRoleURL = "/user-roles"
	registerURL = "/register"
)

var (
	jsonErrConflict        = jsonError("Data already exists!")
	jsonErrInvalidJSONBody = jsonError("Body is not valid JSON")
	jsonErrEmptyBody       = jsonError("Body should not empty")

	validatorTranslator ut.Translator
)

func init() {
	binding.Validator = &ginvalidator.Validator{
		ConfigFn: configureValidation,
	}
}

func (c FieldError) Error() string {
	buff := bytes.Buffer{}
	for _, fieldName := range c {
		buff.WriteString(c[fieldName])
		buff.WriteByte('\n')
	}

	return strings.TrimSpace(buff.String())
}

// LoadRoutes into router
func LoadRoutes(router *gin.Engine) {
	router.Use(ErrorMiddleware)

	LoadAuthRoutes(router)
	LoadUserRoutes(router)
	LoadUserRoleRoutes(router)
}

// ErrorMiddleware handling error after all handler
func ErrorMiddleware(ctx *gin.Context) {
	ctx.Next()

	lastError := ctx.Errors.Last()
	if lastError == nil {
		return
	}

	// checking is error handled or not
	switch lastError.Type {
	case gin.ErrorTypeBind:
		switch err := lastError.Err.(type) {
		case validator.ValidationErrors:
			resJSON := FieldError{}
			for _, fieldError := range err {
				resJSON[fieldError.Field()] = fieldError.Translate(validatorTranslator)
			}
			ctx.String(http.StatusBadRequest, jsonFail(resJSON))
		default:
			ctx.String(http.StatusBadRequest, jsonErrEmptyBody)
		}

	case gin.ErrorTypePrivate:
		switch lastError.Err {
		case gorm.ErrRecordNotFound:
			ctx.String(http.StatusNotFound, jsonError("data not found"))
		default:
			goto CHECK_ERROR_TYPE
		}
		break

	CHECK_ERROR_TYPE:
		_ = "coverage test line"
		switch err := lastError.Err.(type) {
		case *mysql.MySQLError:
			switch err.Number {
			case 1062:
				ctx.JSON(http.StatusConflict, jsonErrConflict)
			}
		default:
			internalServerError(ctx, lastError.Err)
		}

	default:
		internalServerError(ctx, lastError.Err)
	}
}

func internalServerError(ctx *gin.Context, err error) {
	ctx.String(
		http.StatusInternalServerError,
		jsonError("Internal server error"),
	)
	log.Println(
		aurora.BrightRed("[Error]"),
		aurora.BrightRed(err),
	)
}

func configureValidation(v *validator.Validate) {
	translator := en.New()
	validatorTranslator, _ = ut.New(translator, translator).GetTranslator("en")
	en_translations.RegisterDefaultTranslations(v, validatorTranslator)
}

func jsonError(str string) string {
	return `{"status":"error","message":"` + str + `"}`
}

func jsonSuccess(data interface{}) string {
	str, _ := jsoniter.MarshalToString(data)
	return `{"status":"success","data":` + str + `}`
}

func jsonFail(data interface{}) string {
	str, _ := jsoniter.MarshalToString(data)
	return `{"status":"fail","data":` + str + `}`
}

func mustParseUintParam(ctx *gin.Context, key string, bitSize int) (uint64, error) {
	res, err := strconv.ParseUint(ctx.Param(key), 10, bitSize)
	if err != nil {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest,
			jsonFail(map[string]string{
				key: key + " is not a number value",
			}),
		)
		return res, err
	}
	return res, nil
}
