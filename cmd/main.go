package main

import "github.com/gin-gonic/gin"

func main() {
	router := gin.Default()
	router.GET("/", func(context *gin.Context) {
		context.JSON(200, gin.H{
			"message": "Welcome to WebPartyTime!",
		})
	})

	router.Run("0.0.0.0:8080")
}
