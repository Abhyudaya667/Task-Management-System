package controllers

import (
	"net/http"
	"task-management-system/service"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	usersvc service.UserService
}

func NewUserController(svc service.UserService) *UserController{
	return &UserController{usersvc: svc}
}

func (uc *UserController)SearchEmailsByText(c *gin.Context){
	requesterID, ok := getRequesterID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	text := c.Query("search")
	if text == ""{
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "search query required",
		})
		return
	}
	emails,err := uc.usersvc.SearchEmails(c.Request.Context(),requesterID,text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": emails,
	})

}
