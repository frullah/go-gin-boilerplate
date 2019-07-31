package controllers

import (
	"errors"
	"net/http"
	"time"

	"github.com/frullah/gin-boilerplate/db"

	"github.com/jinzhu/gorm"

	"github.com/dgrijalva/jwt-go"
	"github.com/frullah/gin-boilerplate/models"
	"github.com/gin-gonic/gin"
)

// AuthLoginParams login parameters
type AuthLoginParams struct {
	Username string `json:"username" binding:"required,min=5,max=64"`
	Password string `json:"password" binding:"required,min=5,max=64"`
}

// AuthLoginResponse ...
type AuthLoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// JWTClaims struct
type JWTClaims struct {
	jwt.StandardClaims
	UserID uint64 `json:"id"`
}

const (
	accessTokenDuration  = 10 * time.Minute
	refreshTokenDuration = 7 * (24 * time.Hour)

	accessTokenHeader  = "X-Access-Token"
	refreshTokenHeader = "X-Refresh-Token"
)

var (
	accessTokenSecret  = []byte("auth-token-secret")
	refreshTokenSecret = []byte("refresh-token-secret")

	refreshTokenChecker = jwtCheck(refreshTokenSecret)
	accessTokenChecker  = jwtCheck(accessTokenSecret)

	errForbidden          = errors.New("Resource access is forbidden")
	errUnauthorized       = errors.New("Authorization is needed")
	errInvalidTokenMethod = jwt.NewValidationError(
		"Invalid method",
		jwt.ValidationErrorUnverifiable,
	)
	errEmptyToken = errors.New("Empty token")

	jsonErrInvalidAuthorization = makeSimpleJSONError("invalid authorization")
	jsonErrNeedAuthorization    = makeSimpleJSONError("need authorization")
	jsonErrUserDisabled         = makeSimpleJSONError("user disabled")
)

// LoadAuthRoutes to engine
func LoadAuthRoutes(router *gin.Engine) {
	group := router.Group("/auth")
	group.POST("/login", AuthLogin)
}

// AuthLogin handle POST /auth/login
func AuthLogin(ctx *gin.Context) {
	body := AuthLoginParams{}

	if err := ctx.BindJSON(&body); err != nil {
		return
	}

	user := models.User{}
	if err := db.Get(db.Default).
		Select("id, password, is_enabled").
		First(&user, "username = ?", body.Username).
		Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.AbortWithStatus(http.StatusUnauthorized)
		} else {
			abortWithError(ctx, err, gin.ErrorTypePrivate)
		}
		return
	}

	if !user.IsEnabled {
		abortWithString(ctx, http.StatusForbidden, jsonErrUserDisabled)
		return
	}

	if !comparePassword([]byte(user.Password), []byte(body.Password)) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ctx.JSON(http.StatusOK, AuthLoginResponse{
		AccessToken:  makeAccessToken(user.ID),
		RefreshToken: makeRefreshToken(user.ID, user.Password),
	})
}

// AuthRolesMiddleware function
func AuthRolesMiddleware(roles map[string]struct{}) func(*gin.Context) {
	return func(ctx *gin.Context) {

	}
}

// AuthMiddleware ...
func AuthMiddleware(ctx *gin.Context) {
	decoded, err := shouldParseJWT(ctx, accessTokenHeader, accessTokenChecker)
	if err == errEmptyToken || err == errInvalidTokenMethod || decoded == nil {
		return
	}
	accessClaims := decoded.Claims.(*JWTClaims)
	if err != nil {
		decoded, err := shouldParseJWT(ctx, refreshTokenHeader, refreshTokenChecker)
		if err != nil || decoded == nil || !decoded.Valid {
			return
		}
		refreshClaims := decoded.Claims.(*JWTClaims)
		if refreshClaims.UserID != accessClaims.UserID {
			abortWithString(ctx, http.StatusUnauthorized, jsonErrInvalidAuthorization)
			return
		}

		user := models.User{Role: &models.UserRole{}}
		if err := db.Get(db.Default).
			Select("is_enabled, password, role_id").
			Joins("JOIN user_role ON user_role.id = user.role_id").
			First(&user, refreshClaims.UserID).
			Error; err != nil {
			abortWithError(ctx, err, gin.ErrorTypePrivate)
			return
		}
		if !user.IsEnabled {
			abortWithString(ctx, http.StatusUnauthorized, jsonErrUserDisabled)
			return
		}
		ctx.Header(accessTokenHeader, makeAccessToken(accessClaims.UserID))
		ctx.Header(
			refreshTokenHeader,
			makeRefreshToken(refreshClaims.UserID, user.Password),
		)
	}

	ctx.Set("userID", accessClaims.UserID)
	ctx.Next()
}

func shouldParseJWT(ctx *gin.Context, headerKey string, tokenChecker jwt.Keyfunc) (*jwt.Token, error) {
	token := ctx.GetHeader(headerKey)
	if token == "" {
		abortWithString(ctx, http.StatusUnauthorized, jsonErrNeedAuthorization)
		return nil, errEmptyToken
	}
	decoded, err := jwt.ParseWithClaims(token, &JWTClaims{}, tokenChecker)
	if _, ok := err.(*jwt.ValidationError); ok {
		abortWithString(ctx, http.StatusUnauthorized, jsonErrInvalidAuthorization)
	}
	return decoded, err
}

func jwtCheck(secret []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errInvalidTokenMethod
		}
		return secret, nil
	}
}
