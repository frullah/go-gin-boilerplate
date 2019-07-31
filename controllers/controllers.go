package controllers

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/dgrijalva/jwt-go"
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

// CreateDataResponse - response of new data
type CreateDataResponse struct {
	ID int `json:"id"`
}

// CreateBigDataResponse response of new big data
type CreateBigDataResponse struct {
	ID uint64 `json:"id"`
}

// FieldError -
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// FieldErrors is array of FieldError pointer
type FieldErrors []*FieldError

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
	bcryptCost = 12

	jsonErrConflict        = makeSimpleJSONError("Data already exists!")
	jsonErrInvalidJSONBody = makeSimpleJSONError("Body is not valid JSON")
	jsonErrEmptyBody       = makeSimpleJSONError("Body should not empty")

	validatorTranslator ut.Translator
)

func init() {
	binding.Validator = &ginvalidator.Validator{
		ConfigFn: configureValidation,
	}
}

func (c FieldErrors) Error() string {
	buff := bytes.Buffer{}
	for _, fieldError := range c {
		buff.WriteString(fieldError.Message)
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

	handled := true

	// checking is error handled or not
	switch lastError.Type {
	case gin.ErrorTypeBind:
		switch err := lastError.Err.(type) {
		case validator.ValidationErrors:
			resJSON := make(FieldErrors, len(err))
			for i, fieldError := range err {
				resJSON[i] = &FieldError{
					Field:   fieldError.Field(),
					Message: fieldError.Translate(validatorTranslator),
				}
			}
			ctx.JSON(http.StatusBadRequest, resJSON)
		default:
			ctx.String(http.StatusBadRequest, jsonErrEmptyBody)
		}

	case gin.ErrorTypePrivate:
		switch lastError.Err {
		case gorm.ErrRecordNotFound:
			ctx.String(http.StatusNotFound, `{"message":"Data not found"}`)
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
			handled = false
		}

	default:
		handled = false
	}

	if !handled {
		ctx.String(
			http.StatusInternalServerError,
			`{"status":"internal server error"}`,
		)
		log.Println(
			aurora.BrightRed("[Error]"),
			aurora.BrightRed(lastError.Err),
		)
	}
}

func abortWithString(ctx *gin.Context, code int, format string, args ...interface{}) {
	ctx.String(code, format, args...)
	ctx.Abort()
}

func abortWithError(ctx *gin.Context, err error, errorType gin.ErrorType) {
	ctx.Error(err).SetType(errorType)
	ctx.Abort()
}

func configureValidation(v *validator.Validate) {
	translator := en.New()
	validatorTranslator, _ = ut.New(translator, translator).GetTranslator("en")
	en_translations.RegisterDefaultTranslations(v, validatorTranslator)
}

func makeJWT(userID uint64, duration time.Duration, secret []byte) string {
	currentTime := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, JWTClaims{
		jwt.StandardClaims{
			ExpiresAt: currentTime.Add(duration).Unix(),
			IssuedAt:  currentTime.Unix(),
		},
		userID,
	})

	tokenString, _ := token.SignedString(secret)
	return tokenString
}

func makeAccessToken(userID uint64) string {
	return makeJWT(userID, accessTokenDuration, accessTokenSecret)
}

func makeRefreshToken(userID uint64, pwHash string) string {
	return makeJWT(userID, refreshTokenDuration, append(refreshTokenSecret, pwHash...))
}

func comparePassword(hashedPassword, password []byte) bool {
	err := bcrypt.CompareHashAndPassword(hashedPassword, password)
	return err == nil
}

func abortInvalidParamValue(ctx *gin.Context, key string) {
	ctx.AbortWithStatusJSON(
		http.StatusBadRequest,
		`{"error":{"message":"param "`+key+`" is not a valid value"}}`,
	)
}

func makeSimpleJSONError(str string) string {
	return `{"error":{"message":"` + str + `"}}`
}

func parseUintParam(ctx *gin.Context, key string, bitSize int) (uint64, error) {
	res, err := strconv.ParseUint(ctx.Param(key), 10, bitSize)
	if err != nil {
		abortInvalidParamValue(ctx, key)
		return res, err
	}
	return res, nil
}
