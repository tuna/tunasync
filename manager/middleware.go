package manager

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func contextErrorLogger(c *gin.Context) {
	errs := c.Errors.ByType(gin.ErrorTypeAny)
	if len(errs) > 0 {
		for _, err := range errs {
			logger.Error(`"in request "%s %s: %s"`,
				c.Request.Method, c.Request.URL.Path,
				err.Error())
		}
	}
	// pass on to the next middleware in chain
	c.Next()
}

func (s *managerServer) workerIDValidator(c *gin.Context) {
	workerID := c.Param("id")
	_, err := s.adapter.GetWorker(workerID)
	if err != nil {
		// no worker named `workerID` exists
		err := fmt.Errorf("invalid workerID %s", workerID)
		s.returnErrJSON(c, http.StatusBadRequest, err)
		c.Abort()
		return
	}
	// pass on to the next middleware in chain
	c.Next()
}
