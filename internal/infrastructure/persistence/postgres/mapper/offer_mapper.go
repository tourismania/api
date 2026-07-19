package mapper

import (
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToOfferDomain converts a sqlc row to a domain entity.
func ToOfferDomain(row db.Offer) entity.Offer {
	return entity.Offer{
		ID:          int(row.ID),
		UUID:        row.Uuid,
		Title:       row.Title,
		Description: row.Description,
		AgencyID:    int(row.AgencyID),
		CreatedBy:   int(row.CreatedBy),
		Status:      enum.OfferStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	}
}

// ToOfferDomainFromListRow converts a ListOffers row to a domain entity.
func ToOfferDomainFromListRow(row db.ListOffersRow) entity.Offer {
	return entity.Offer{
		ID:          int(row.ID),
		UUID:        row.Uuid,
		Title:       row.Title,
		Description: row.Description,
		AgencyID:    int(row.AgencyID),
		CreatedBy:   int(row.CreatedBy),
		Status:      enum.OfferStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	}
}
