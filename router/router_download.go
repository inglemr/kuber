package router

import (
	"bufio"
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kubectyl/kuber/router/middleware"
	"github.com/kubectyl/kuber/router/tokens"
	"github.com/kubectyl/kuber/server/backup"
)

// Handle a download request for a server backup.
func getDownloadBackup(c *gin.Context) {
	client := middleware.ExtractApiClient(c)
	manager := middleware.ExtractManager(c)

	token := tokens.BackupPayload{}
	if err := tokens.ParseToken([]byte(c.Query("token")), &token); err != nil {
		middleware.CaptureAndAbort(c, err)
		return
	}

	if _, ok := manager.Get(token.ServerUuid); !ok || !token.IsUniqueRequest() {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "The requested resource was not found on this server.",
		})
		return
	}

	b, st, err := backup.LocateLocal(client, token.BackupUuid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "The requested backup was not found on this server.",
			})
			return
		}

		middleware.CaptureAndAbort(c, err)
		return
	}

	f, err := os.Open(b.Path())
	if err != nil {
		middleware.CaptureAndAbort(c, err)
		return
	}
	defer f.Close()

	c.Header("Content-Length", strconv.Itoa(int(st.Size())))
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(st.Name()))
	c.Header("Content-Type", "application/octet-stream")

	_, _ = bufio.NewReader(f).WriteTo(c.Writer)
}

// Handles downloading a specific file for a server.
func getDownloadFile(c *gin.Context) {
	manager := middleware.ExtractManager(c)
	token := tokens.FilePayload{}
	if err := tokens.ParseToken([]byte(c.Query("token")), &token); err != nil {
		middleware.CaptureAndAbort(c, err)
		return
	}

	s, ok := manager.Get(token.ServerUuid)
	if !ok || !token.IsUniqueRequest() {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "The requested resource was not found on this server.",
		})
		return
	}

	p, _ := s.Filesystem().SafePath(token.FilePath)
	st, err := os.Stat(p)
	// If there is an error or we're somehow trying to download a directory, just
	// respond with the appropriate error.
	if err != nil {
		middleware.CaptureAndAbort(c, err)
		return
	} else if st.IsDir() {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "The requested resource was not found on this server.",
		})
		return
	}

	f, err := os.Open(p)
	if err != nil {
		middleware.CaptureAndAbort(c, err)
		return
	}

	c.Header("Content-Length", strconv.Itoa(int(st.Size())))
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(st.Name()))
	c.Header("Content-Type", "application/octet-stream")

	_, _ = bufio.NewReader(f).WriteTo(c.Writer)
}
