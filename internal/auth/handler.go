package auth

import (
	"github.com/baskararestu/wms-api/internal/pkg/response"
	"github.com/baskararestu/wms-api/internal/pkg/validation"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for the Auth domain
type Handler struct {
	service Service
}

// NewHandler creates a new Auth Handler instance
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes binds the handler methods to the Fiber router
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/login", validation.New[LoginRequest](), h.Login)
	router.Post("/refresh", validation.New[RefreshTokenRequest](), h.RefreshToken)
	router.Post("/logout", validation.New[LogoutRequest](), h.Logout)
}

// Login handles the user authentication and token generation
// Login godoc
// @Summary Login user
// @Description Authenticate an internal user and return access and refresh tokens.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login payload"
// @Success 200 {object} response.SuccessResponse{data=LoginResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
	// The validation middleware safely injected our strongly-typed struct
	req := c.Locals("payload").(*LoginRequest)

	res, err := h.service.Login(*req)
	if err != nil {
		xlogger.Logger.Warn().Str("email", req.Email).Err(err).Msg("Login failed")
		return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
			Code:    fiber.StatusUnauthorized,
			Message: err.Error(),
		})
	}

	xlogger.Logger.Info().Str("email", req.Email).Msg("User logged in successfully")
	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Login successful",
		Data:    res,
	})
}

// RefreshToken handles access token renewal using a refresh token
// RefreshToken godoc
// @Summary Refresh access token
// @Description Exchange a valid refresh token for a new access token pair.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token payload"
// @Success 200 {object} response.SuccessResponse{data=LoginResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/auth/refresh [post]
func (h *Handler) RefreshToken(c *fiber.Ctx) error {
	req := c.Locals("payload").(*RefreshTokenRequest)

	res, err := h.service.RefreshToken(*req)
	if err != nil {
		xlogger.Logger.Warn().Err(err).Msg("Refresh token failed")
		return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
			Code:    fiber.StatusUnauthorized,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Token refreshed successfully",
		Data:    res,
	})
}

// Logout invalidates a refresh token session
// Logout godoc
// @Summary Logout user
// @Description Revoke a refresh token session.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LogoutRequest true "Logout payload"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/auth/logout [post]
func (h *Handler) Logout(c *fiber.Ctx) error {
	req := c.Locals("payload").(*LogoutRequest)

	err := h.service.Logout(*req)
	if err != nil {
		xlogger.Logger.Warn().Err(err).Msg("Logout failed")
		return c.Status(fiber.StatusUnauthorized).JSON(response.ErrorResponse{
			Code:    fiber.StatusUnauthorized,
			Message: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessResponse{
		Code:    fiber.StatusOK,
		Message: "Logout successful",
	})
}
