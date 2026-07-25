package db

import (
	"time"

	"github.com/google/uuid"
)

// User is the sqlc-style row representation. Nullable columns use the
// pgx-friendly pointer form; JSON column is decoded into a free-form
// map. Mapping to the domain entity happens in the repository adapter.
type User struct {
	ID               int32
	Uuid             uuid.UUID
	FirstName        *string
	LastName         *string
	Email            string
	Login            string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Phone            *string
	Password         string
	IsActive         bool
	Birthday         *time.Time
	ExtraInformation []byte
	Roles            []string
	AgencyID         int32
}

// Agency is the sqlc-style row representation of the agencies table.
type Agency struct {
	ID        int32
	Uuid      uuid.UUID
	Name      string
	Status    string
	CreatedAt time.Time
	DeletedAt *time.Time
}

// Offer is the sqlc-style row representation of the offers table.
type Offer struct {
	ID          int32
	Uuid        uuid.UUID
	Title       string
	Description string
	AgencyID    int32
	CreatedBy   int32
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// OfferFlight is the sqlc-style row representation of the offer_flights
// table.
type OfferFlight struct {
	ID      int32
	OfferID int32
}

// OfferFlightSegment is the sqlc-style row representation of the
// offer_flight_segments table.
type OfferFlightSegment struct {
	ID                   int32
	FlightID             int32
	Sequence             int16
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	DepartureAt          time.Time
	ArrivalAt            time.Time
}
