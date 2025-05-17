package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func RegisterHelloWorldHandler(e *echo.Echo) {
	e.Add("GET", "hello", HelloWorld)
}

type ErrorMessage struct {
	Err    string `json:"error"`
	Reason string `json:"reason"`
}

func (e *ErrorMessage) Error() string {
	return e.Err
}

func HelloWorld(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":      "OK",
		"description": "all good :)",
	})
}
