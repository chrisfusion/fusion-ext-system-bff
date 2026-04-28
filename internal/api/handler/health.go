package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Health(c *gin.Context) { c.Status(http.StatusOK) }
func Livez(c *gin.Context)  { c.Status(http.StatusOK) }
func Readyz(c *gin.Context) { c.Status(http.StatusOK) }
