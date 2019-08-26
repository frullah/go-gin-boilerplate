package controllers

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"strings"

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

// Response success or failed to the client
// if failed, set Status to "fail" with Data as map
// > which the format is
// > {[field]: "a message which describe why it's failed in validation"}
// else if success, set Status to "success" with Data
type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

// ResponseError to the client
// the keys "Status" ans "Message" is required
type ResponseError struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Code    int         `json:"code,omitempty"`
}

const (
	userURL     = "/users"
	userRoleURL = "/user-roles"
	registerURL = "/register"
)

var (
	jsonErrConflict = ResponseError{
		Status:  "error",
		Message: "Data already exists!",
	}
	jsonErrInvalidJSONBody = ResponseError{
		Status:  "error",
		Message: "Body is not valid JSON",
	}
	jsonErrEmptyBody = ResponseError{
		Status:  "error",
		Message: "Body should not empty",
	}

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
			ctx.PureJSON(http.StatusBadRequest, Response{"fail", resJSON})
		default:
			ctx.PureJSON(http.StatusBadRequest, jsonErrEmptyBody)
		}

	case gin.ErrorTypePrivate:
		switch lastError.Err {
		case gorm.ErrRecordNotFound:
			ctx.PureJSON(http.StatusNotFound, ResponseError{
				Status:  "error",
				Message: "data not found",
			})
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
				ctx.PureJSON(http.StatusConflict, jsonErrConflict)
			}
		default:
			internalServerError(ctx, lastError.Err)
		}

	default:
		internalServerError(ctx, lastError.Err)
	}
}

func internalServerError(ctx *gin.Context, err error) {
	ctx.PureJSON(
		http.StatusInternalServerError,
		&ResponseError{
			Status:  "error",
			Message: "Internal server error",
		},
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

	v.RegisterAlias("username", "min=5,max=64")
	v.RegisterAlias("password", "min=5,max=64")

	betweenTranslator := func(min, max int) validator.TranslationFunc {
		minStr := strconv.Itoa(min)
		maxStr := strconv.Itoa(max)
		minFloat := float64(min)
		maxFloat := float64(max)

		return func(ut ut.Translator, fe validator.FieldError) string {
			var msg string
			valueLen := len(fe.Value().(string))

			if valueLen < min {
				param, _ := ut.C("min-string-character", minFloat, 0, minStr)
				msg, _ = ut.T("min-string", fe.Field(), param)
			}

			if valueLen > max {
				param, _ := ut.C("max-string-character", maxFloat, 0, maxStr)
				msg, _ = ut.T("max-string", fe.Field(), param)
			}

			return msg
		}
	}
	emptyRegisterTranslationFn := func(t ut.Translator) error {
		return nil
	}

	v.RegisterTranslation(
		"username",
		validatorTranslator,
		emptyRegisterTranslationFn,
		betweenTranslator(5, 64),
	)
	v.RegisterTranslation(
		"password",
		validatorTranslator,
		emptyRegisterTranslationFn,
		betweenTranslator(5, 64),
	)
}

func mustParseUintParam(ctx *gin.Context, key string, bitSize int) (uint64, error) {
	res, err := strconv.ParseUint(ctx.Param(key), 10, bitSize)
	if err != nil {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest,
			Response{
				"fail",
				map[string]string{key: key + " is not a number value"},
			},
		)
		return res, err
	}
	return res, nil
}
