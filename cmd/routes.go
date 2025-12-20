package main

import (
	"github.com/gin-gonic/gin"
)

func root(context *gin.Context) {
	context.JSON(200, gin.H{
		"message": "Welcome to WebPartyTime!",
	})
}
