package validation

import (
	"github.com/baskararestu/wms-api/internal/pkg/response"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// New initializes a generic validation middleware for Fiber.
// It parses the request body into V, validates it, and stores the pointer in c.Locals.
func New[V any]() fiber.Handler {
	validate := validator.New(validator.WithRequiredStructEnabled())

	return func(c *fiber.Ctx) error {
		var v V

		if err := c.BodyParser(&v); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
				Code:    fiber.StatusBadRequest,
				Message: "Invalid request body format",
				Errors:  []string{err.Error()},
			})
		}

		if err := validate.Struct(v); err != nil {
			var errors []string
			for _, err := range err.(validator.ValidationErrors) {
				message := err.Field() + " is " + err.Tag()
				if err.Param() != "" {
					message += " " + err.Param()
				}
				errors = append(errors, message)
			}

			return c.Status(fiber.StatusBadRequest).JSON(response.ErrorResponse{
				Code:    fiber.StatusBadRequest,
				Message: "Validation failed",
				Errors:  errors,
			})
		}

		// Store validated payload in locals to be extracted by handler
		c.Locals("payload", &v)
		return c.Next()
	}
}
