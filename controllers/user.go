package controllers

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/AlekSi/pointer"

	"github.com/gin-gonic/gin/binding"

	"github.com/frullah/gin-boilerplate/db"

	"github.com/gin-gonic/gin"

	"github.com/frullah/gin-boilerplate/models"
)

const defaultUserRoleID = 58

// LoadUserRoutes to router
func LoadUserRoutes(engine *gin.Engine) {
	engine.POST(registerURL, UserRegister)
	engine.GET("/user-availibility", UserAvailibility)

	group := engine.Group(userURL)
	authorized := group.Group("")
	authorized.Use(
		AuthRolesMiddleware(map[string]struct{}{
			"administrator": {},
		}),
	)
	authorized.GET(":id", UserGetOne)
	authorized.PUT(":id", UserUpdate)
	authorized.DELETE(":id", UserDelete)
	authorized.POST("", UserCreateOne)
}

// UserAvailibility check the username or email is available to register
// @Success 200 {object} models.User
// @Failure 401
// @Failure 403
// @Router /user-availibility [get]
func UserAvailibility(c *gin.Context) {
	var data interface{}
	qCtx := c.Query("context")
	value := c.Query("value")

	switch strings.ToUpper(qCtx) {
	case "EMAIL":
		data = &struct {
			Value string `json:"value" binding:"required,email"`
		}{value}
	case "USERNAME":
		data = &struct {
			Value string `json:"value" binding:"required,username"`
		}{value}
	default:
		c.JSON(http.StatusBadRequest, &Response{
			"fail",
			&struct {
				Context string `json:"context"`
			}{"context must be an email or username"},
		})
		c.Abort()
		return
	}

	if err := binding.Validator.ValidateStruct(data); err != nil {
		c.Error(err).SetType(gin.ErrorTypeBind)
		c.Abort()
		return
	}

	exists := false
	query := "SELECT 1 FROM `user` WHERE " + qCtx + " = ?"
	err := db.Get(db.Default).Raw(query, value).Row().Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, Response{
		"success",
		&struct {
			Available bool `json:"available"`
		}{!exists},
	})
}

// UserGetOne docs
// @Success 200 {object} models.User
// @Failure 401
// @Failure 403
// @Router /users [get]
func UserGetOne(ctx *gin.Context) {
	id, err := mustParseUintParam(ctx, "id", 64)
	if err != nil {
		return
	}

	user := models.User{Role: &models.UserRole{}}
	if err := db.Get(db.Default).
		Select("id, email, username, name, enabled").
		First(&user, id).
		Related(user.Role, "Role").
		Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", &user})
}

// UserCreateOne docs
// @Accept json
// @Success 200 {object} models.User
// @Failure 401
// @Failure 403
// @Router /users [post]
func UserCreateOne(ctx *gin.Context) {
	data := struct {
		Email    string `json:"email" binding:"required,email"`
		Username string `json:"username" binding:"required,username"`
		Password string `json:"password" binding:"required,password"`
		Name     string `json:"name" binding:"required,max=64"`
		RoleID   uint32 `json:"roleId" binding:"required,min=1"`
		Enabled  bool   `json:"enabled"`
	}{}
	if err := ctx.BindJSON(&data); err != nil {
		return
	}

	user := models.User{
		Email:    data.Email,
		Username: data.Username,
		Password: data.Password,
		Name:     data.Name,
		RoleID:   data.RoleID,
		Enabled:  data.Enabled,
		Verified: true,
	}
	if err := db.Get(db.Default).
		Model(&user).
		Create(&user).
		Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", Uint64ID{*user.ID}})
}

// UserUpdate docs
// @Accept json
// @Param id path int true "User ID"
// @Param body body models.User true "User ID"
// @Success 200 {object} models.User
// @Failure 401
// @Failure 403
// @Router /users/{id} [put]
func UserUpdate(ctx *gin.Context) {
	id, err := mustParseUintParam(ctx, "id", 64)
	if err != nil {
		return
	}

	body := struct {
		Email    string `json:"email,omitempty" binding:"omitempty,email"`
		Username string `json:"username,omitempty" binding:"omitempty,username"`
		Password string `json:"password,omitempty" binding:"omitempty,password"`
		Name     string `json:"name,omitempty" binding:"omitempty,max=64"`
		RoleID   uint32 `json:"roleId,omitempty" binding:"omitempty,min=1"`
		Enabled  bool   `json:"enabled,omitempty"`
	}{}
	ctx.ShouldBindJSON(&body)

	updatedUser := models.User{
		ID:       pointer.ToUint64(id),
		Email:    body.Email,
		Username: body.Username,
		Password: body.Password,
		Name:     body.Name,
		RoleID:   body.RoleID,
		Enabled:  body.Enabled,
	}
	if err := db.Get(db.Default).
		Model(&updatedUser).
		UpdateColumns(&updatedUser).
		Error; err != nil {
		log.Println(err)
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", nil})
}

// UserDelete docs
// @Param id path int true "User ID"
// @Success 200 {object} models.User
// @Failure 401
// @Failure 403
// @Router /users{id} [delete]
func UserDelete(ctx *gin.Context) {
	id, err := mustParseUintParam(ctx, "id", 64)
	if err != nil {
		return
	}

	if err := db.Get(db.Default).
		Delete(&models.User{}, id).
		Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", nil})
}

// UserRegister docs
// @Success 200 {object} models.User
// @Failure 401
// @Router /users/register [post]
func UserRegister(ctx *gin.Context) {
	data := struct {
		Email    string `json:"email" binding:"required,email"`
		Username string `json:"username" binding:"required,username"`
		Password string `json:"password" binding:"required,password"`
		Name     string `json:"name" binding:"required,max=64"`
	}{}
	if err := ctx.BindJSON(&data); err != nil {
		return
	}

	newUser := models.User{
		Email:    data.Email,
		Username: data.Username,
		Password: data.Password,
		Name:     data.Name,
		RoleID:   defaultUserRoleID,
	}
	if err := db.Get(db.Default).
		Create(&newUser).
		Error; err != nil {
		ctx.Error(err)
		return
	}

	ctx.PureJSON(http.StatusOK, Response{"success", IntID{int(*newUser.ID)}})
}
