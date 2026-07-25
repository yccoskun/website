// Package handlers contains thin HTTP handlers; business logic lives in
// services as it appears in later phases.
package handlers

import (
	"net/http"

	"github.com/yccoskun/website/internal/response"
)

// Health reports server liveness.
func Health(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
