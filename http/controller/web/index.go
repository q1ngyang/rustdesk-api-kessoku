package web

import (
	"github.com/gin-gonic/gin"
)

type Index struct {
}

func (i *Index) Index(c *gin.Context) {
	c.Redirect(302, "/dash/")
}
