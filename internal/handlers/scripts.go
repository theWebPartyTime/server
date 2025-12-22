package handlers

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/theWebPartyTime/server/internal/models"
	"github.com/theWebPartyTime/server/internal/service"

	"github.com/gin-gonic/gin"
)

type ScriptsHandler struct {
	scriptsService *service.ScriptsService
}

func NewScriptsHandler(scriptService *service.ScriptsService) *ScriptsHandler {
	return &ScriptsHandler{scriptsService: scriptService}
}

func (h *ScriptsHandler) UserScripts(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid limit parameter",
		})
		return
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid offset parameter",
		})
		return
	}
	u, ok := getUserFromContext(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}
	searchQuery := c.DefaultQuery("search", "")
	scripts, err := h.scriptsService.GetUserScripts(c.Request.Context(), u.ID, limit, offset, searchQuery)
	if err != nil {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "error retrieving data from the database",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"scripts": scripts,
	})
}

func (h *ScriptsHandler) PublicScripts(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid limit parameter",
		})
		return
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid offset parameter",
		})
		return
	}
	searchQuery := c.DefaultQuery("search", "")
	scripts, err := h.scriptsService.GetPublicScripts(c.Request.Context(), limit, offset, searchQuery)
	if err != nil {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "error retrieving data from the database",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scripts": scripts,
	})

}

func (h *ScriptsHandler) UploadScript(c *gin.Context) {
	var scriptRequest models.CreateScript
	scriptFile, err := c.FormFile("script")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "script file is required"})
		return
	}

	f, err := scriptFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
	scriptRequest.ScriptFile = f
	coverFile, err := c.FormFile("cover")
	var coverReader io.Reader
	if coverFile != nil && err != nil {
		cf, err := coverFile.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cf.Close()
		coverReader = cf
	}
	scriptRequest.CoverFile = coverReader

	public, err := strconv.ParseBool(c.PostForm("public"))
	if err != nil {
		public = false
		c.JSON(http.StatusBadRequest, gin.H{"error": "script file is required"})
		return
	}

	title := c.PostForm("title")
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	description := c.PostForm("description")
	if description == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "description is required"})
		return
	}

	scriptRequest.Public = public
	scriptRequest.Title = title
	scriptRequest.Description = description

	u, ok := getUserFromContext(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	scriptRequest.CreatorId = u.ID

	h.scriptsService.UploadScript(c.Request.Context(), scriptRequest)

	c.JSON(http.StatusOK, gin.H{
		"message": "Script uploaded successfully",
	})

}

func (h *ScriptsHandler) UpdateScript(c *gin.Context) {
	var updateRequest models.UpdateScript

	scriptHash := c.Param("script_hash")
	if scriptHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "script_hash path parameter is required"})
		return
	}

	script, err := h.scriptsService.GetScriptByHash(c.Request.Context(), scriptHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	u, ok := getUserFromContext(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	if script.CreatorId != u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	scriptFileHeader, err := c.FormFile("script")
	var scriptReader io.Reader
	if err == nil && scriptFileHeader != nil {
		f, err := scriptFileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer f.Close()
		scriptReader = f
	}

	coverFileHeader, err := c.FormFile("cover")
	var coverReader io.Reader
	if err == nil && coverFileHeader != nil {
		cf, err := coverFileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cf.Close()
		coverReader = cf
	}

	title := c.PostForm("title")
	if title == "" {
		title = script.Title
	}
	description := c.PostForm("description")
	if description == "" {
		description = script.Description
	}
	publicStr := c.PostForm("public")
	var public bool
	if publicStr != "" {
		public, _ = strconv.ParseBool(publicStr)
	} else {
		public = script.Public
	}

	updateRequest = models.UpdateScript{
		ScriptFile:  scriptReader,
		CoverFile:   coverReader,
		Title:       title,
		Description: description,
		Public:      public,
	}

	err = h.scriptsService.UpdateScript(c.Request.Context(), script.ScriptHash, script.CoverHash, updateRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "script updated"})

}

func getUserFromContext(c *gin.Context) (*models.User, bool) {
	u, ok := c.Get("user")
	if !ok {
		return nil, false
	}

	user, ok := u.(*models.User)
	return user, ok
}
