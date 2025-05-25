package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/kobu/repo-int/internal/model"
	"github.com/kobu/repo-int/pkg/c8yauth"
	est "github.com/kobu/repo-int/pkg/externalstorage"
	"github.com/labstack/echo/v4"
	"github.com/reubenmiller/go-c8y/pkg/c8y"
)

var estClient *est.ExternalStorageClient

func RegisterFirmwareHandler(e *echo.Echo, eClient *est.ExternalStorageClient) {
	estClient = eClient
	e.Add("GET", "firmware/download", DownloadFileViaRedirect, c8yauth.Authorization(c8yauth.RoleDevice))
}

type ErrorMessage struct {
	Err    string `json:"error"`
	Reason string `json:"reason"`
}

func (e *ErrorMessage) Error() string {
	return e.Err
}

func DownloadFileViaRedirect(c echo.Context) error {
	cc := c.(*model.RequestContext)

	auth, err := c8yauth.GetUserSecurityContext(c)
	if err != nil {
		return c.JSON(http.StatusForbidden, ErrorMessage{
			Err:    "invalid user context",
			Reason: err.Error(),
		})
	}

	id := c.QueryParam("id")
	if len(id) == 0 {
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{
			"status":  http.StatusUnprocessableEntity,
			"message": "Missing 'id' parameter in request",
		})
	}
	presignedUrl, statusCode, content := GeneratePresignedUrl(cc.Microservice.WithServiceUser(auth.Tenant), cc.Microservice.Client, id)
	if statusCode != http.StatusOK {
		return c.JSON(statusCode, content)
	}
	return c.Redirect(http.StatusTemporaryRedirect, presignedUrl)
}

func GeneratePresignedUrl(ctx context.Context, c8yClient *c8y.Client, moid string) (string, int, map[string]any) {
	// query Managed Object
	mo, resp, err := c8yClient.Inventory.GetManagedObject(ctx, moid, nil)
	if err != nil {
		slog.Error("Error while getting the Managed Object", "err", err.Error())
		if resp.StatusCode() == 404 {
			return "", http.StatusNotFound, map[string]any{
				"status":  http.StatusNotFound,
				"message": "No Managed Object found for id=" + moid,
			}
		} else {
			return "", http.StatusInternalServerError, map[string]any{
				"status":  http.StatusInternalServerError,
				"message": "Error while getting Managed Object wit id=" + moid,
				"error":   err.Error(),
			}
		}
	}
	// extract reference to external storage
	objectKey := mo.Item.Get("externalResourceOrigin.objectKey").String()
	if len(objectKey) == 0 {
		slog.Error("Firmware Managed Object does not contain 'externalResourceOrigin.objectKey'", "managedObjectId", mo.ID)
		return "", http.StatusUnprocessableEntity, map[string]any{
			"status":  http.StatusUnprocessableEntity,
			"message": "Missing 'externalResourceOrigin.objectKey' on Managed Object id '" + moid + "'",
		}
	}
	// generate presigned URL
	presignedUrl, err := (*estClient).GetPresignedURL(objectKey)
	if err != nil {
		slog.Error("Error while generating presigned URL for objectKey", "objectKey", objectKey, "err", err.Error())
		return "", http.StatusInternalServerError, map[string]any{
			"status":  http.StatusInternalServerError,
			"message": "Error while generating presigned URL for objectKey='" + objectKey + "' from Managed Object '" + mo.ID + "'",
			"error":   err.Error(),
		}
	}
	return presignedUrl, http.StatusOK, map[string]any{
		"url": presignedUrl,
	}
}
