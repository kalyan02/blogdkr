package main

import (
	"github.com/gin-gonic/gin"
)

// auth middleware
var authKey = &struct{}{}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// If authentication is successful, set a value in the context
		c.Set("authKey", authKey)
	}
}

func IsAuthenticated(c *gin.Context) bool {
	val, exists := c.Get("authKey")
	if !exists {
		return false
	}
	if val != authKey {
		return false
	}
	return true
}
