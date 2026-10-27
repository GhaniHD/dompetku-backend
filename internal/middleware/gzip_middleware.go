package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var gzipWriters = sync.Pool{New: func() any {
	w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
	return w
}}

type gzipWriter struct {
	gin.ResponseWriter
	gz    *gzip.Writer
	wrote bool
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	w.wrote = true
	return w.gz.Write(b)
}

func (w *gzipWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func (w *gzipWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

// Gzip mengompres respons JSON API. File Excel tidak dikompres ulang karena
// formatnya sudah berupa zip.
func Gzip() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api/") || strings.HasSuffix(path, ".xlsx") ||
			!strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") || c.Request.Method == http.MethodHead {
			c.Next()
			return
		}

		gz := gzipWriters.Get().(*gzip.Writer)
		gz.Reset(c.Writer)
		h := c.Writer.Header()
		h.Set("Content-Encoding", "gzip")
		h.Add("Vary", "Accept-Encoding")
		w := &gzipWriter{ResponseWriter: c.Writer, gz: gz}
		c.Writer = w

		defer func() {
			if w.wrote {
				gz.Close()
			} else {
				// Tanpa body (mis. 204): jangan kirim footer gzip.
				h.Del("Content-Encoding")
			}
			gz.Reset(nil)
			gzipWriters.Put(gz)
		}()
		c.Next()
	}
}
