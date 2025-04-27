package handlers

import (
	"net/http"

	"github.com/reubenmiller/go-c8y/pkg/microservice"

	"github.com/kobu/repo-int/pkg/c8yauth"
	"github.com/labstack/echo/v4"
)

// RegisterHandlers registers the http handlers to the given echo server
func RegisterCertificateHandlers(e *echo.Echo) {
	// e.Add("GET", "hello", HelloWorld, c8yauth.Authorization(c8yauth.RoleDevice))
	e.Add("GET", "hello", HelloWorld)
}

func ExternalIdExists(m *microservice.Microservice, tenant string, externalID string) bool {
	// Check for proof that the external id definitely does NOT exist
	_, extResp, _ := m.Client.Identity.GetExternalID(
		m.WithServiceUser(tenant),
		"c8y_Serial",
		externalID,
	)
	return extResp != nil && extResp.StatusCode() == http.StatusOK
}

type ErrorMessage struct {
	Err    string `json:"error"`
	Reason string `json:"reason"`
}

func (e *ErrorMessage) Error() string {
	return e.Err
}

func HelloWorld(c echo.Context) error {
	// cc := c.(*model.RequestContext)

	// checking if user starts with device_:
	_, err := c8yauth.GetUserSecurityContext(c)
	if err != nil {
		return c.JSON(http.StatusForbidden, ErrorMessage{
			Err:    "invalid user context",
			Reason: err.Error(),
		})
	}
	// externalID := strings.TrimPrefix(auth.UserID, "device_")
	// if externalID == "" {
	// 	slog.Error("Could not derive external name from user.", "userID", auth.UserID)
	// 	return c.JSON(http.StatusUnprocessableEntity, ErrorMessage{
	// 		Err:    "Invalid user id detected in token",
	// 		Reason: "The request must be a device user and not any other type of user",
	// 	})
	// }

	return c.JSON(http.StatusOK, map[string]any{
		"status":      "OK",
		"description": "all good :)",
	})
}
