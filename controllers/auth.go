package controllers

import (
	"errors"
	"net/http"
	"time"

	"github.com/frullah/gin-boilerplate/db"
	"golang.org/x/crypto/bcrypt"

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
	bcryptCost = 12

	accessTokenSecret  = []byte("auth-token-secret")
	refreshTokenSecret = []byte("refresh-token-secret")

	refreshTokenChecker = jwtCheck(refreshTokenSecret)
	accessTokenChecker  = jwtCheck(accessTokenSecret)

	errInvalidTokenMethod = jwt.NewValidationError(
		"Invalid method",
		jwt.ValidationErrorUnverifiable,
	)
	errEmptyToken = errors.New("Empty token")

	jsonErrInvalidAuthorization = jsonError("Invalid authorization")
	jsonErrUnauthorized         = jsonError("Unauthorized")
	jsonErrUserDisabled         = jsonError("User disabled")
)

// LoadAuthRoutes to engine
func LoadAuthRoutes(router *gin.Engine) {
	group := router.Group("/auth")
	group.POST("/login", AuthLogin)
	group.GET("/google/v2/")
}

// AuthLogin handle POST /auth/login
func AuthLogin(ctx *gin.Context) {
	body := AuthLoginParams{}

	if err := ctx.BindJSON(&body); err != nil {
		return
	}

	user := models.User{}
	if err := db.Get(db.Default).
		Select("id, password, enabled").
		First(&user, "username = ?", body.Username).
		Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.AbortWithStatus(http.StatusUnauthorized)
		} else {
			ctx.Error(err)
			ctx.Abort()
		}
		return
	}

	if !user.Enabled {
		ctx.String(http.StatusForbidden, jsonErrUserDisabled)
		ctx.Abort()
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
func AuthRolesMiddleware(allowedRoles map[string]struct{}) func(*gin.Context) {
	return func(ctx *gin.Context) {
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
				ctx.String(http.StatusUnauthorized, jsonErrInvalidAuthorization)
				ctx.Abort()
				return
			}

			user := models.User{}
			if err := db.Get(db.Default).
				Select("enabled, password").
				First(&user, refreshClaims.UserID).
				Error; err != nil {
				ctx.Error(err)
				ctx.Abort()
				return
			}

			if !user.Enabled {
				ctx.String(http.StatusUnauthorized, jsonErrUserDisabled)
				ctx.Abort()
				return
			}

			ctx.Header(accessTokenHeader, makeAccessToken(accessClaims.UserID))
			ctx.Header(
				refreshTokenHeader,
				makeRefreshToken(refreshClaims.UserID, user.Password),
			)
		}

		if allowedRoles != nil {
			role := models.UserRole{}
			db.Get(db.Default).
				Table("user").
				Select("user_role.name").
				Joins("INNER JOIN user ON user_role.id = user.role_id").
				First(&role, accessClaims.UserID)
			if _, ok := allowedRoles[role.Name]; !ok {
				return
			}
		}

		ctx.Set("userID", accessClaims.UserID)
		ctx.Next()
	}
}

func shouldParseJWT(ctx *gin.Context, headerKey string, tokenChecker jwt.Keyfunc) (*jwt.Token, error) {
	token := ctx.GetHeader(headerKey)
	if token == "" {
		ctx.String(http.StatusUnauthorized, jsonErrUnauthorized)
		ctx.Abort()
		return nil, errEmptyToken
	}
	decoded, err := jwt.ParseWithClaims(token, &JWTClaims{}, tokenChecker)
	if _, ok := err.(*jwt.ValidationError); ok {
		ctx.String(http.StatusUnauthorized, jsonErrInvalidJSONBody)
		ctx.Abort()
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

func comparePassword(hashedPassword, password []byte) bool {
	err := bcrypt.CompareHashAndPassword(hashedPassword, password)
	return err == nil
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
