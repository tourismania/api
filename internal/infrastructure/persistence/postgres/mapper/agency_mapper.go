package mapper

import (
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToAgencyDomain converts a sqlc row to a domain entity.
func ToAgencyDomain(row db.Agency) entity.Agency {
	return entity.Agency{
		ID:        int(row.ID),
		UUID:      row.Uuid,
		Name:      row.Name,
		Status:    enum.AgencyStatus(row.Status),
		CreatedAt: row.CreatedAt,
		DeletedAt: row.DeletedAt,
	}
}
