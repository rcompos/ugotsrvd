package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rcompos/ugotsrvd"
)

func main() {

	// //Load the .env file
	// err := godotenv.Load(".env")
	// if err != nil {
	// 	log.Println("error: failed to load the env file")
	// }

	if os.Getenv("ENV") == "PRODUCTION" {
		gin.SetMode(gin.ReleaseMode)
	}
	fmt.Println("GITHUB_TOKEN:", os.Getenv("GITHUB_TOKEN"))

	ugotsrvd.LogEnvVars()

	router := gin.Default()
	// Set a lower memory limit for multipart forms (default is 32 MiB)
	router.MaxMultipartMemory = 8 << 20 // 8 MiB

	// router.LoadHTMLGlob("views/*")
	router.LoadHTMLGlob("templates/*")
	// router.Static("/", "./public")
	router.Static("/upload", "./public")
	// router.Static("/package", "./public")

	// router.GET("/clusters/:id", ugotsrvd.GetClusterByID)
	// router.POST("/clusters", ugotsrvd.PostClusters) // create new cluster config
	// router.GET("/", ugotsrvd.IndexHandler)

	router.POST("/upload", ugotsrvd.Upload)
	router.GET("/package", ugotsrvd.Package)
	router.POST("/create", ugotsrvd.Create)
	router.GET("/array", ugotsrvd.GetArray)      // Testing templates
	router.GET("/listfiles", ugotsrvd.ListFiles) // List CAPI YAML files

	router.Run(":8080")
}
