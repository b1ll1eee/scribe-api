package http

import (
	"net/http"
	"strconv"
	"strings"

	"scribe/backend/internal/domain/user"
	"scribe/backend/internal/ports"

	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	authService      ports.AuthService
	scribeService      ports.ScribeService
	analyticsService ports.AnalyticsService
}

func NewHttpHandler(auth ports.AuthService, scribes ports.ScribeService, analytics ports.AnalyticsService) *HttpHandler {
	return &HttpHandler{
		authService:      auth,
		scribeService:      scribes,
		analyticsService: analytics,
	}
}

type LoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type CreateScribeRequest struct {
	Title   string   `json:"title" binding:"required,max=255"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type UpdateScribeRequest struct {
	Title   string   `json:"title" binding:"required,max=255"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

func jsonOK(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{"success": true, "data": data, "error": nil})
}

func jsonErr(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"success": false, "data": nil, "error": gin.H{"code": code, "message": message}})
}

func currentUser(c *gin.Context) *user.User {
	u, _ := c.Get("user")
	return u.(*user.User)
}

// --- Auth Handlers ---

func (h *HttpHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonErr(c, http.StatusBadRequest, "INVALID_REQUEST", "id_token is required")
		return
	}

	u, accessToken, refreshToken, err := h.authService.LoginWithFirebase(c.Request.Context(), req.IDToken)
	if err != nil {
		jsonErr(c, http.StatusUnauthorized, "AUTH_FAILED", "Authentication failed")
		return
	}

	c.SetCookie("refresh_token", refreshToken, 7*24*3600, "/api/v1/auth", "", false, true)
	jsonOK(c, http.StatusOK, gin.H{
		"user":          u,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *HttpHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	_ = c.ShouldBindJSON(&req)

	rfToken := req.RefreshToken
	if rfToken == "" {
		if t, err := c.Cookie("refresh_token"); err == nil {
			rfToken = t
		}
	}

	if rfToken == "" {
		jsonErr(c, http.StatusBadRequest, "INVALID_TOKEN", "Refresh token is missing")
		return
	}

	accessToken, newRefreshToken, err := h.authService.RefreshToken(c.Request.Context(), rfToken)
	if err != nil {
		jsonErr(c, http.StatusUnauthorized, "REFRESH_FAILED", "Token refresh failed")
		return
	}

	c.SetCookie("refresh_token", newRefreshToken, 7*24*3600, "/api/v1/auth", "", false, true)
	jsonOK(c, http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

func (h *HttpHandler) Logout(c *gin.Context) {
	var req RefreshRequest
	_ = c.ShouldBindJSON(&req)

	rfToken := req.RefreshToken
	if rfToken == "" {
		if t, err := c.Cookie("refresh_token"); err == nil {
			rfToken = t
		}
	}

	if rfToken != "" {
		_ = h.authService.Logout(c.Request.Context(), rfToken)
	}

	c.SetCookie("refresh_token", "", -1, "/api/v1/auth", "", false, true)
	jsonOK(c, http.StatusOK, gin.H{"message": "Successfully logged out"})
}

func (h *HttpHandler) Me(c *gin.Context) {
	u := currentUser(c)
	profile, err := h.authService.GetUserProfile(c.Request.Context(), u.ID)
	if err != nil {
		jsonErr(c, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}
	jsonOK(c, http.StatusOK, profile)
}

// --- Scribe Handlers ---

func (h *HttpHandler) CreateScribe(c *gin.Context) {
	u := currentUser(c)
	var req CreateScribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonErr(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	n, err := h.scribeService.CreateScribe(c.Request.Context(), u.ID, req.Title, req.Content, req.Tags)
	if err != nil {
		jsonErr(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}
	jsonOK(c, http.StatusCreated, n)
}

func (h *HttpHandler) UpdateScribe(c *gin.Context) {
	u := currentUser(c)
	scribeID := c.Param("id")

	var req UpdateScribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonErr(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	n, err := h.scribeService.UpdateScribe(c.Request.Context(), u.ID, scribeID, req.Title, req.Content, req.Tags)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonErr(c, http.StatusNotFound, "SCRIBE_NOT_FOUND", "Scribe not found")
			return
		}
		jsonErr(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	jsonOK(c, http.StatusOK, n)
}

func (h *HttpHandler) GetScribe(c *gin.Context) {
	u := currentUser(c)
	scribeID := c.Param("id")

	n, err := h.scribeService.GetScribe(c.Request.Context(), scribeID, u.ID)
	if err != nil {
		jsonErr(c, http.StatusNotFound, "SCRIBE_NOT_FOUND", "Scribe not found")
		return
	}
	jsonOK(c, http.StatusOK, n)
}

func (h *HttpHandler) DeleteScribe(c *gin.Context) {
	u := currentUser(c)
	scribeID := c.Param("id")

	if err := h.scribeService.DeleteScribe(c.Request.Context(), scribeID, u.ID); err != nil {
		jsonErr(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	jsonOK(c, http.StatusOK, gin.H{"message": "Scribe deleted successfully"})
}

func (h *HttpHandler) RestoreScribe(c *gin.Context) {
	u := currentUser(c)
	scribeID := c.Param("id")

	n, err := h.scribeService.RestoreScribe(c.Request.Context(), scribeID, u.ID)
	if err != nil {
		jsonErr(c, http.StatusInternalServerError, "RESTORE_FAILED", err.Error())
		return
	}
	jsonOK(c, http.StatusOK, n)
}

func (h *HttpHandler) ArchiveScribe(c *gin.Context) {
	u := currentUser(c)
	scribeID := c.Param("id")

	n, err := h.scribeService.ArchiveScribe(c.Request.Context(), scribeID, u.ID)
	if err != nil {
		jsonErr(c, http.StatusBadRequest, "ARCHIVE_FAILED", err.Error())
		return
	}
	jsonOK(c, http.StatusOK, n)
}

func (h *HttpHandler) PinScribe(c *gin.Context) {
	u := currentUser(c)
	scribeID := c.Param("id")

	n, err := h.scribeService.TogglePinScribe(c.Request.Context(), scribeID, u.ID)
	if err != nil {
		jsonErr(c, http.StatusBadRequest, "PIN_FAILED", err.Error())
		return
	}
	jsonOK(c, http.StatusOK, n)
}

func (h *HttpHandler) ListScribes(c *gin.Context) {
	u := currentUser(c)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	search := c.Query("search")
	tagsStr := c.Query("tags")

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var tags []string
	if strings.TrimSpace(tagsStr) != "" {
		tags = strings.Split(tagsStr, ",")
	}

	params := ports.ScribeListParams{
		OwnerID:   u.ID,
		Search:    search,
		Tags:      tags,
		Limit:     limit,
		Offset:    offset,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	if v := c.Query("is_archived"); v != "" {
		b, _ := strconv.ParseBool(v)
		params.IsArchived = &b
	}
	if v := c.Query("is_pinned"); v != "" {
		b, _ := strconv.ParseBool(v)
		params.IsPinned = &b
	}
	if v := c.Query("include_deleted"); v != "" {
		b, _ := strconv.ParseBool(v)
		params.IncludeDeleted = b
	}

	scribes, total, err := h.scribeService.ListScribes(c.Request.Context(), params)
	if err != nil {
		jsonErr(c, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	jsonOK(c, http.StatusOK, gin.H{
		"scribes":  scribes,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// --- Analytics Handler ---

func (h *HttpHandler) GetDashboard(c *gin.Context) {
	u := currentUser(c)
	dashboard, err := h.analyticsService.GetDashboard(c.Request.Context(), u.ID)
	if err != nil {
		jsonErr(c, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
		return
	}
	jsonOK(c, http.StatusOK, dashboard)
}
