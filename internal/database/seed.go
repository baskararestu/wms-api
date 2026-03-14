package database

import (
	"github.com/baskararestu/wms-api/internal/auth"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedAll runs the database seeding process
func SeedAll(db *gorm.DB) {
	seedAdminUser(db)
}

func seedAdminUser(db *gorm.DB) {
	var count int64
	db.Model(&auth.User{}).Where("email = ?", "admin@wms.com").Count(&count)
	
	if count == 0 {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := &auth.User{
			Email:        "admin@wms.com",
			PasswordHash: string(hashedPassword),
		}
		
		if err := db.Create(admin).Error; err != nil {
			xlogger.Logger.Fatal().Err(err).Msg("Failed to seed admin user")
		}
		xlogger.Logger.Info().Msg("✅ Admin user seeded successfully! (admin@wms.com / admin123)")
	}
}
