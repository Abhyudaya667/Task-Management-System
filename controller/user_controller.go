package controllers

import (
	"net/http"
	"task-management-system/service"

	"github.com/gin-gonic/gin"
	"errors"

	"task-management-system/repository"
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

func (uc *UserController) SearchLabels(c *gin.Context) {
	text := c.Query("search")
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "search query required"})
		return
	}

	labels, err := uc.usersvc.SearchLabels(c.Request.Context(), text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": labels})
}

func (uc *UserController) AddNewLabel(c *gin.Context) {
	requesterID, ok := getRequesterID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var body struct {
		Label string `json:"label" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newLabel, err := uc.usersvc.AddLabel(c.Request.Context(), requesterID, body.Label)
	if err != nil {
		if errors.Is(err, repository.ErrLabelExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "label already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": newLabel})
}
