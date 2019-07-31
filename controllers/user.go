package controllers

import (
	"net/http"

	"github.com/frullah/gin-boilerplate/db"

	// "gopkg.in/go-playground/validator.v9"

	"github.com/gin-gonic/gin"

	"github.com/frullah/gin-boilerplate/models"
)

// UserPostParams struct
type UserPostParams struct {
	Email     string `json:"email" binding:"required,email"`
	Username  string `json:"username" binding:"required,min=5,max=64"`
	Password  string `json:"password" binding:"required,min=5,max=64"`
	Name      string `json:"name" binding:"required,max=64"`
	RoleID    uint32 `json:"roleId" binding:"required,min=1"`
	IsEnabled bool   `json:"isEnabled"`
}

// UserRegisterParams struct
type UserRegisterParams struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=5,max=64"`
	Password string `json:"password" binding:"required,min=5,max=64"`
	Name     string `json:"name" binding:"required,max=64"`
}

// UserUpdateBody struct
type UserUpdateBody struct {
	Email     string `json:"email,omitempty" binding:"omitempty,email"`
	Username  string `json:"username,omitempty" binding:"omitempty,min=5,max=64"`
	Password  string `json:"password,omitempty" binding:"omitempty,min=5,max=64"`
	Name      string `json:"name,omitempty" binding:"omitempty,max=64"`
	RoleID    uint32 `json:"roleId,omitempty" binding:"omitempty,min=1"`
	IsEnabled bool   `json:"isEnabled,omitempty"`
}

// UserUpdateParams struct
type UserUpdateParams struct {
	ID uint64 `uri:"id"`
	UserUpdateBody
}

// LoadUserRoutes to router
func LoadUserRoutes(engine *gin.Engine) {
	engine.POST(registerURL, UserRegister)

	group := engine.Group(userURL)
	authorized := group.Group("")
	authorized.Use(AuthMiddleware)
	authorized.GET(":id", UserGetOne)
	authorized.PUT(":id", UserUpdate)
	authorized.DELETE(":id", UserDelete)
	authorized.POST("", UserCreateOne)
}

// UserGetOne handle GET /users/:id
func UserGetOne(ctx *gin.Context) {
	id, err := parseUintParam(ctx, "id", 64)
	if err != nil {
		return
	}

	user := &models.User{Role: &models.UserRole{}}
	if err := db.Get(db.Default).
		First(user, id).
		Related(user.Role, "Role").
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}

	ctx.JSON(http.StatusOK, user)
}

// UserCreateOne handler POST /users
func UserCreateOne(ctx *gin.Context) {
	data := UserPostParams{}
	if err := ctx.BindJSON(&data); err != nil {
		return
	}

	user := models.User{
		Email:     data.Email,
		Username:  data.Username,
		Password:  data.Password,
		Name:      data.Name,
		RoleID:    data.RoleID,
		IsEnabled: data.IsEnabled,
		Verified:  true,
	}
	if err := db.Get(db.Default).
		Model(&user).
		Create(&user).
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}

	ctx.JSON(http.StatusOK, CreateBigDataResponse{ID: user.ID})
}

// UserUpdate handle PUT /users/:id
func UserUpdate(ctx *gin.Context) {
	id, err := parseUintParam(ctx, "id", 64)
	if err != nil {
		return
	}

	body := UserUpdateBody{}
	ctx.ShouldBindJSON(&body)

	updatedUser := models.User{
		ID:        id,
		Email:     body.Email,
		Username:  body.Username,
		Password:  body.Password,
		Name:      body.Name,
		RoleID:    body.RoleID,
		IsEnabled: body.IsEnabled,
	}
	if err := db.Get(db.Default).
		Model(&updatedUser).
		UpdateColumns(&updatedUser).
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}
}

// UserDelete handle DELETE /users/:id
func UserDelete(ctx *gin.Context) {
	id, err := parseUintParam(ctx, "id", 64)
	if err != nil {
		return
	}

	if err := db.Get(db.Default).
		Delete(&models.User{}, id).
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}
}

// UserRegister handles POST /users/register
func UserRegister(ctx *gin.Context) {
	data := UserRegisterParams{}
	if err := ctx.BindJSON(&data); err != nil {
		return
	}

	newUser := models.User{
		Email:    data.Email,
		Username: data.Username,
		Password: data.Password,
		Name:     data.Name,
	}
	if err := db.Get(db.Default).
		Model(&newUser).
		Create(&newUser).
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}

	ctx.JSON(http.StatusOK, CreateDataResponse{ID: int(newUser.ID)})
}
