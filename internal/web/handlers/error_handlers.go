package handlers

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
)

// HandleNotFound handles 404 Not Found errors
func (h *Handlers) HandleNotFound(c *gin.Context, title string, message string) {
	if title == "" {
		title = "Page Not Found"
	}
	if message == "" {
		message = "The page you are looking for does not exist."
	}
	h.HandleError(c, 404, title, message, nil)
}

// HandleBadRequest handles 400 Bad Request errors
func (h *Handlers) HandleBadRequest(c *gin.Context, title, message string) {
	h.HandleError(c, 400, title, message, nil)
}

// HandleUnauthorized handles 401 Unauthorized errors
func (h *Handlers) HandleUnauthorized(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)
	_ = components.UnauthorizedError(ctx).Render(ctx, c.Writer)
}

// HandleServerError handles 500 Internal Server errors
func (h *Handlers) HandleServerError(c *gin.Context, err error) {
	var details string
	if err != nil {
		details = err.Error()
	}
	ctx := components.CreateTemplateContext(c)
	_ = components.ServerError(ctx, details).Render(ctx, c.Writer)
}

// HandleError handles generic errors with custom title and message
func (h *Handlers) HandleError(c *gin.Context, code int, title, message string, err error) {
	var details string
	if err != nil {
		details = err.Error()
	}
	ctx := components.CreateTemplateContext(c)
	_ = components.ErrorPage(ctx, code, title, message, details).Render(ctx, c.Writer)
}

// RegisterErrorHandlers sets up custom error handlers for the router
func (h *Handlers) RegisterErrorHandlers(router *gin.Engine) {
	// Register 404 handler
	router.NoRoute(func(c *gin.Context) {
		h.HandleNotFound(c, "", "")
	})

	// Register 405 handler (Method Not Allowed)
	router.NoMethod(func(c *gin.Context) {
		h.HandleError(c, 405, "Method Not Allowed",
			fmt.Sprintf("The %s method is not supported for this resource.", c.Request.Method), nil)
	})

	// Register function to recover from panics
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		var err error
		if rec, ok := recovered.(error); ok {
			err = rec
		} else {
			err = fmt.Errorf("%v", recovered)
		}
		h.HandleServerError(c, err)
		c.Abort()
	}))
}
