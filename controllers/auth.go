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

// JWTClaims struct
type JWTClaims struct {
	jwt.StandardClaims
	UserID uint64 `json:"id"`
}

const (
	accessTokenDuration  = 10 * time.Minute
	refreshTokenDuration = 7 * (24 * time.Hour)

	// AccessTokenHeader ...
	AccessTokenHeader = "X-Access-Token"
	// RefreshTokenHeader ...
	RefreshTokenHeader = "X-Refresh-Token"
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

	jsonErrInvalidAuthorization = &ResponseError{
		Status:  "error",
		Message: "Invalid authorization",
	}
	jsonErrUnauthorized = &ResponseError{
		Status:  "error",
		Message: "Unauthorized",
	}
	jsonErrUserDisabled = &ResponseError{
		Status:  "error",
		Message: "User disabled",
	}
)

// LoadAuthRoutes to engine
func LoadAuthRoutes(router *gin.Engine) {
	group := router.Group("/auth")
	group.POST("/login", AuthLogin)
	// group.GET("/google/v2")

	authenticated := group.Group("")
	authenticated.Use(AuthRolesMiddleware(nil))
	authenticated.GET("data", AuthData)
}

// AuthLogin handler
// @Success 200 {object} struct{AccessToken string}
// @Failure 401
// @Failure 403 ResponseError
// @Router /auth/login [post]
func AuthLogin(ctx *gin.Context) {
	body := struct {
		Username string `json:"username" binding:"username"`
		Password string `json:"password" binding:"password"`
	}{}
	if err := ctx.BindJSON(&body); err != nil {
		return
	}

	user := models.User{}
	if err := db.Get(db.Default).
		Select("id, password, enabled").
		First(&user, "username = ?", body.Username).
		Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
		} else {
			ctx.Error(err)
			ctx.Abort()
		}

		return
	}

	if !user.Enabled {
		ctx.PureJSON(http.StatusForbidden, jsonErrUserDisabled)
		ctx.Abort()
		return
	}

	if !comparePassword([]byte(user.Password), []byte(body.Password)) {
		ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}{
		AccessToken:  makeAccessToken(*user.ID),
		RefreshToken: makeRefreshToken(*user.ID),
	})
}

// AuthData retrieve data from authenticated user
// @Success 200 {object} type struct{}
// @Failure 401
// @Router /auth/data [get]
func AuthData(ctx *gin.Context) {
	userID := ctx.MustGet("userID").(uint64)
	user := models.User{Role: &models.UserRole{}}
	if err := db.Get(db.Default).
		Select("enabled, username, name").
		First(&user, userID).
		Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
		} else {
			ctx.Error(err)
		}
		return
	}

	if !user.Enabled {
		ctx.PureJSON(http.StatusForbidden, jsonErrUserDisabled)
		return
	}

	userRole := models.UserRole{}
	db.Get(db.Default).Select("enabled, name").First(&userRole, user.RoleID)
	if !userRole.Enabled {
		ctx.PureJSON(http.StatusForbidden, jsonErrUserDisabled)
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", &struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Role     string `json:"role"`
	}{user.Username, user.Name, userRole.Name}})
}

// AuthRolesMiddleware function
func AuthRolesMiddleware(allowedRoles map[string]struct{}) func(*gin.Context) {
	return func(ctx *gin.Context) {
		decoded, err := parseAccessToken(ctx)
		if err == errEmptyToken || err == errInvalidTokenMethod || decoded == nil {
			ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
			ctx.Abort()
			return
		}

		shouldCreateNewToken := false
		accessClaims := decoded.Claims.(*JWTClaims)
		if err != nil {
			decoded, err := parseRefreshToken(ctx)
			if err != nil || decoded == nil || !decoded.Valid {
				ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
				ctx.Abort()
				return
			}

			refreshClaims := decoded.Claims.(*JWTClaims)
			if refreshClaims.UserID != accessClaims.UserID {
				ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
				ctx.Abort()
				return
			}

			user := models.User{}
			if err := db.Get(db.Default).
				Select("enabled, password").
				First(&user, refreshClaims.UserID).
				Error; err != nil {
				ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
				ctx.Abort()
				return
			}

			if !user.Enabled {
				ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
				ctx.Abort()
				return
			}

			shouldCreateNewToken = true
		}

		if allowedRoles != nil {
			role := models.UserRole{}
			db.Get(db.Default).
				Table("user").
				Select("user_role.name").
				Joins("INNER JOIN user ON user_role.id = user.role_id").
				First(&role, accessClaims.UserID)
			if _, ok := allowedRoles[role.Name]; !ok {
				ctx.PureJSON(http.StatusUnauthorized, jsonErrUnauthorized)
				ctx.Abort()
				return
			}
		}

		if shouldCreateNewToken {
			ctx.Header(AccessTokenHeader, makeAccessToken(accessClaims.UserID))
			ctx.Header(RefreshTokenHeader, makeRefreshToken(accessClaims.UserID))
		}

		ctx.Set("userID", accessClaims.UserID)
		ctx.Next()
	}
}

func parseJWT(ctx *gin.Context, headerKey string, tokenChecker jwt.Keyfunc) (*jwt.Token, error) {
	token := ctx.GetHeader(headerKey)
	if token == "" {
		return nil, errEmptyToken
	}
	return jwt.ParseWithClaims(token, &JWTClaims{}, tokenChecker)
}

func parseAccessToken(ctx *gin.Context) (*jwt.Token, error) {
	return parseJWT(ctx, AccessTokenHeader, accessTokenChecker)
}

func parseRefreshToken(ctx *gin.Context) (*jwt.Token, error) {
	return parseJWT(ctx, RefreshTokenHeader, refreshTokenChecker)
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
			IssuedAt:  currentTime.Unix(),
			ExpiresAt: currentTime.Add(duration).Unix(),
		},
		userID,
	})

	tokenString, _ := token.SignedString(secret)
	return tokenString
}

func makeAccessToken(userID uint64) string {
	return makeJWT(userID, accessTokenDuration, accessTokenSecret)
}

func makeRefreshToken(userID uint64) string {
	return makeJWT(userID, refreshTokenDuration, refreshTokenSecret)
}
