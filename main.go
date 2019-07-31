package main

import (
	"fmt"

	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"

	"github.com/frullah/gin-boilerplate/fs"
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
// @version 1.0
// @description This is a sample gin app.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host http://localhost:3000
// @BasePath /
func main() {
	fmt.Println("Initializing server...")
	gin.SetMode(gin.ReleaseMode)

	fs.InitAsOS()
	config.Init()
	db.Init()
	defer db.Close()

	cnf := config.Get()
	port := cnf.Server.Port
	if port == 0 {
		port = 8080
	}

	host := fmt.Sprintf("%s:%d", cnf.Server.Host, port)

	router := gin.Default()
	router.Use(cors.Default())
	controllers.LoadRoutes(router)

	swaggerURL := ginSwagger.URL(fmt.Sprintf("http://%s/swagger/doc.json", host))
	swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler, swaggerURL)
	router.GET("/swagger/*any", swaggerHandler)

	fmt.Println("Server initialized!")
	fmt.Println("Server running on " + host + "\n")

	router.Run(host)
}
