package main

import (
	"fmt"
	"os"

	"github.com/logrusorgru/aurora"

	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"

	"github.com/frullah/gin-boilerplate/fs"
	"github.com/frullah/gin-boilerplate/models"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/frullah/gin-boilerplate/config"
	"github.com/frullah/gin-boilerplate/controllers"
	"github.com/frullah/gin-boilerplate/db"
	_ "github.com/frullah/gin-boilerplate/docs"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/swaggo/files"
	_ "github.com/swaggo/gin-swagger"
)

// @title Swagger Example API
// @version 2.0
// @description This is a sample gin app.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host http://localhost:3000
// @BasePath /
func main() {
	fmt.Println("Initializing server...")

	fs.InitAsOS()
	if err := config.Init(); err != nil {
		panic(err)
	}
	if err := db.Init(); err != nil {
		panic(err)
	}

	defer db.Close()

	cnf := config.Get()
	port := cnf.Server.Port
	if port == 0 {
		port = 8080
	}

	host := fmt.Sprintf("%s:%d", cnf.Server.Host, port)

	// initialize router
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// initialize cors
	corsConfig := cors.DefaultConfig()
	corsConfig.AddAllowHeaders(
		controllers.AccessTokenHeader,
		controllers.RefreshTokenHeader,
	)
	corsConfig.AllowAllOrigins = true

	router.Use(cors.New(corsConfig))
	controllers.LoadRoutes(router)
	if os.Getenv("APP_ENV") == "test" {
		fmt.Println(aurora.Yellow("Running in testing mode"))

		// add reset db handler for test environment
		router.POST("/db/user/reset", func(ctx *gin.Context) {
			db.Get(db.Default).Delete(&models.User{})
		})
	}

	swaggerURL := ginSwagger.URL(fmt.Sprintf("http://%s/swagger/doc.json", host))
	swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler, swaggerURL)
	router.GET("/swagger/*any", swaggerHandler)

	fmt.Println(aurora.BrightGreen("Server initialized!"))
	fmt.Println("Server running on", aurora.BrightBlue(host))
	fmt.Println()

	router.Run(host)
}
