package api

import "github.com/gofiber/fiber/v2"

// APIv1 retrieves all the APIv1 routes
func APIv1(server *fiber.App) {
	v1 := server.Group("/api/v1")
	userRoutes(v1)
	listingRoutes(v1)
	urlsRoutes(v1)
}
