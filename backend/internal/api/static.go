package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"dockeradmin/web"
)

func (s *Server) registerStatic(r *gin.Engine) {
	distFS := web.Dist
	indexBytes, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		s.log.Error("embed index.html missing", "err", err)
		return
	}
	fileServer := http.FileServer(http.FS(distFS))

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			respondErr(c, http.StatusNotFound, "not_found", "接口不存在", nil)
			return
		}
		if strings.Contains(path.Base(p), ".") {
			clean := strings.TrimPrefix(p, "/")
			if f, err := distFS.Open(clean); err == nil {
				_ = f.Close()
				if strings.HasPrefix(p, "/assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	})
}
