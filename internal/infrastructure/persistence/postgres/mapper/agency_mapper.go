package mapper

import (
	"api/internal/domain/agency"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToAgencyDomain converts a sqlc row to a domain entity.
func ToAgencyDomain(row db.Agency) agency.Agency {
	return agency.Agency{
		ID:        int(row.ID),
		UUID:      row.Uuid,
		Name:      row.Name,
		Status:    agency.Status(row.Status),
		CreatedAt: row.CreatedAt,
		DeletedAt: row.DeletedAt,
	}
}
