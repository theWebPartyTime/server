package handlers

import (
	"io"
	"log"
	"net/http"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
)

type AssetsHandler struct {
	assetsStorage storage.FilesStorage
	contentType   string
}

func NewAssetsHandler(storage storage.FilesStorage, contentType string) *AssetsHandler {
	return &AssetsHandler{assetsStorage: storage, contentType: contentType}
}

func (h *AssetsHandler) GetMediaByHash(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hash required"})
		return
	}

	file, err := h.assetsStorage.Open(c.Request.Context(), hash)
	if err != nil {
		log.Println("Error opening file:", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	defer file.Close()

	c.Header("Content-Type", h.contentType)
	if _, err := io.Copy(c.Writer, file); err != nil {
		log.Println("Error writing file to response:", err)
	}
}
