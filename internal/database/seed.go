package database

import (
	"github.com/baskararestu/wms-api/internal/auth"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	usershops "github.com/baskararestu/wms-api/internal/user-shops"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedAll runs the database seeding process
func SeedAll(db *gorm.DB) {
	seedAdminUser(db)
}

func seedAdminUser(db *gorm.DB) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		xlogger.Logger.Fatal().Err(err).Msg("Failed to hash admin password")
	}

	admin := &auth.User{}
	err = db.Where("email = ?", "admin@wms.com").First(admin).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			xlogger.Logger.Fatal().Err(err).Msg("Failed to query admin user")
		}

		admin = &auth.User{
			Email:        "admin@wms.com",
			PasswordHash: string(hashedPassword),
		}
		if err := db.Create(admin).Error; err != nil {
			xlogger.Logger.Fatal().Err(err).Msg("Failed to seed admin user")
		}
		xlogger.Logger.Info().Msg("✅ Admin user seeded successfully! (admin@wms.com / admin123)")
	}

	userShop := &usershops.UserShop{}
	err = db.Where("user_id = ? AND shop_id = ?", admin.ID, "shopee-123").First(userShop).Error
	if err == gorm.ErrRecordNotFound {
		userShop = &usershops.UserShop{
			UserID: admin.ID,
			ShopID: "shopee-123",
		}
		if err := db.Create(userShop).Error; err != nil {
			xlogger.Logger.Fatal().Err(err).Msg("Failed to seed user-shop relationship")
		}
		xlogger.Logger.Info().Msg("✅ User-shop relationship seeded successfully! (admin@wms.com -> shopee-123)")
		return
	}

	if err != nil {
		xlogger.Logger.Fatal().Err(err).Msg("Failed to query user-shop relationship")
	}
}
