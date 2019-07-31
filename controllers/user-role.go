package controllers

import (
	"net/http"

	"github.com/frullah/gin-boilerplate/db"
	"github.com/frullah/gin-boilerplate/models"
	"github.com/gin-gonic/gin"
)

// UserRoleCreateOneBody -
type UserRoleCreateOneBody struct {
	Name      string `json:"name" binding:"required"`
	IsEnabled bool   `json:"isEnabled"`
}

// UserRoleUpdateBody -
type UserRoleUpdateBody struct {
	Name      string `json:"name" binding:"required"`
	IsEnabled bool   `json:"isEnabled"`
}

// UserRoleCreateOne handle POST: /user-roles
func UserRoleCreateOne(ctx *gin.Context) {
	data := UserRoleCreateOneBody{}
	if err := ctx.BindJSON(&data); err != nil {
		return
	}

	user := models.UserRole{
		Name:      data.Name,
		IsEnabled: data.IsEnabled,
	}
	if err := db.Get(db.Default).
		Model(&user).
		Create(&user).Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}

	ctx.JSON(http.StatusOK, CreateDataResponse{ID: int(user.ID)})

}

// UserRoleUpdate handle PUT /user-roles/:id
func UserRoleUpdate(ctx *gin.Context) {
	id, err := parseUintParam(ctx, "id", 32)
	if err != nil {
		return
	}

	body := UserUpdateBody{}
	ctx.ShouldBindJSON(&body)

	updatedUser := &models.UserRole{
		ID:        uint32(id),
		Name:      body.Name,
		IsEnabled: body.IsEnabled,
	}
	if err := db.Get(db.Default).
		Model(updatedUser).
		UpdateColumns(updatedUser).
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}
}

// UserRoleDelete handle DELETE /user-roles/:id
func UserRoleDelete(ctx *gin.Context) {
	id, err := parseUintParam(ctx, "id", 32)
	if err != nil {
		return
	}

	if err := db.Get(db.Default).
		Delete(&models.UserRole{}, uint32(id)).
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}
}

// UserRoleGetOne handle GET /user-roles/:id
func UserRoleGetOne(ctx *gin.Context) {
	id, err := parseUintParam(ctx, "id", 32)
	if err != nil {
		return
	}

	userRole := &models.UserRole{}
	if err := db.Get(db.Default).
		First(userRole, uint32(id)).
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}

	ctx.JSON(http.StatusOK, userRole)
}

// UserRoleGetMany handle GET /user-roles
func UserRoleGetMany(ctx *gin.Context) {
	userRole := &models.UserRole{}
	if err := db.Get(db.Default).
		Error; err != nil {
		abortWithError(ctx, err, gin.ErrorTypePrivate)
		return
	}

	ctx.JSON(http.StatusOK, userRole)
}

// LoadUserRoleRoutes to router
func LoadUserRoleRoutes(router *gin.Engine) {
	routes := router.Group("/user-roles")
	authorized := routes.Group("")
	authorized.Use(AuthMiddleware)
	authorized.GET("/:id", UserRoleGetOne)
	authorized.PUT("/:id", UserRoleUpdate)
	authorized.DELETE("/:id", UserRoleDelete)
	authorized.POST("", UserRoleCreateOne)
}
