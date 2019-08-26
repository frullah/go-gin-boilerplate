package controllers

import (
	"net/http"

	"github.com/frullah/gin-boilerplate/db"
	"github.com/frullah/gin-boilerplate/models"
	"github.com/gin-gonic/gin"
)

// UserRoleBody ...
type UserRoleBody struct {
	Name    string `json:"name" binding:"required"`
	Enabled bool   `json:"enabled"`
}

// UserRoleCreateOne handle POST: /user-roles
func UserRoleCreateOne(ctx *gin.Context) {
	data := UserRoleBody{}
	if err := ctx.BindJSON(&data); err != nil {
		return
	}

	user := models.UserRole{
		Name:    data.Name,
		Enabled: data.Enabled,
	}
	if err := db.Get(db.Default).
		Model(&user).
		Create(&user).Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", IntID{int(user.ID)}})
}

// UserRoleUpdate handle PUT /user-roles/:id
func UserRoleUpdate(ctx *gin.Context) {
	id, err := mustParseUintParam(ctx, "id", 32)
	if err != nil {
		return
	}

	body := UserRoleBody{}
	ctx.ShouldBindJSON(&body)

	updatedUser := &models.UserRole{
		ID:      uint32(id),
		Name:    body.Name,
		Enabled: body.Enabled,
	}
	if err := db.Get(db.Default).
		Model(updatedUser).
		UpdateColumns(updatedUser).
		Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", nil})
}

// UserRoleDelete handle DELETE /user-roles/:id
func UserRoleDelete(ctx *gin.Context) {
	id, err := mustParseUintParam(ctx, "id", 32)
	if err != nil {
		return
	}

	if err := db.Get(db.Default).
		Delete(&models.UserRole{}, uint32(id)).
		Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", nil})
}

// UserRoleGetOne handle GET /user-roles/:id
func UserRoleGetOne(ctx *gin.Context) {
	id, err := mustParseUintParam(ctx, "id", 32)
	if err != nil {
		return
	}

	userRole := &models.UserRole{}
	if err := db.Get(db.Default).
		First(userRole, uint32(id)).
		Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{"success", userRole})
}

// UserRoleGetMany handle GET /user-roles
func UserRoleGetMany(ctx *gin.Context) {
	userRoles := []models.UserRole{}
	count := uint64(0)
	defaultDB := db.Get(db.Default)

	find := defaultDB.Limit(25).Find(&userRoles)
	if err := find.Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	if err := defaultDB.
		Table("user_role").
		Count(&count).
		Error; err != nil {
		ctx.Error(err)
		ctx.Abort()
		return
	}

	ctx.PureJSON(http.StatusOK, &Response{
		"success",
		&struct {
			Count uint64            `json:"count"`
			Items []models.UserRole `json:"items"`
		}{count, userRoles},
	})
}

// LoadUserRoleRoutes to router
func LoadUserRoleRoutes(router *gin.Engine) {
	routes := router.Group("/user-roles")
	authorized := routes.Group("")
	authorized.Use(AuthRolesMiddleware(map[string]struct{}{
		"administrator": {},
	}))
	authorized.GET("/:id", UserRoleGetOne)
	authorized.GET("", UserRoleGetMany)
	authorized.PUT("/:id", UserRoleUpdate)
	authorized.DELETE("/:id", UserRoleDelete)
	authorized.POST("", UserRoleCreateOne)
}
