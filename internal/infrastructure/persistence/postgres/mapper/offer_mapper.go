package mapper

import (
	"api/internal/domain/offer"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToOfferDomain converts a sqlc row to a domain entity.
func ToOfferDomain(row db.Offer) offer.Offer {
	return offer.Offer{
		ID:          int(row.ID),
		UUID:        row.Uuid,
		Title:       row.Title,
		Description: row.Description,
		AgencyID:    int(row.AgencyID),
		CreatedBy:   int(row.CreatedBy),
		Status:      offer.Status(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	}
}

// ToOfferDomainFromListRow converts a ListOffers row to a domain entity.
func ToOfferDomainFromListRow(row db.ListOffersRow) offer.Offer {
	return offer.Offer{
		ID:          int(row.ID),
		UUID:        row.Uuid,
		Title:       row.Title,
		Description: row.Description,
		AgencyID:    int(row.AgencyID),
		CreatedBy:   int(row.CreatedBy),
		Status:      offer.Status(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	}
}
