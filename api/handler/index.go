package handler

import (
	"fmt"

	"github.com/caitunai/go-blueprint/api/base"
	"github.com/gin-gonic/gin"
)

func HomePage(c *base.Context) {
	user := c.LoginUser()
	if user == nil {
		c.Forbidden("you are not login", gin.H{})
	} else {
		c.Success(gin.H{
			"data": fmt.Sprintf("your user id is: %d", user.ID),
		})
	}
}

func APIHomePage(c *base.Context) {
	user := c.LoginUser()
	if user == nil {
		c.Forbidden("you are not login", gin.H{})
	}
	c.Success(gin.H{
		"user": user,
	})
}
