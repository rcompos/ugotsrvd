package main

import (
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

	ugotsrvd.LogEnvVars()

	router := gin.Default()
	// Set a lower memory limit for multipart forms (default is 32 MiB)
	router.MaxMultipartMemory = 8 << 20 // 8 MiB

	router.LoadHTMLGlob("templates/*")
	router.GET("/", ugotsrvd.IndexHandler)
	router.GET("/upload", ugotsrvd.GetUpload)
	router.POST("/upload", ugotsrvd.PostUpload)
	router.GET("/package", ugotsrvd.Package)
	router.POST("/create", ugotsrvd.Create)
	router.GET("/array", ugotsrvd.GetArray)      // Testing templates
	router.GET("/listfiles", ugotsrvd.ListFiles) // List CAPI YAML files

	router.Run(":8080")
}
