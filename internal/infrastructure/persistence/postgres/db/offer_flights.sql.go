package db

import (
	"context"
	"time"
)

const createOfferFlightSQL = `INSERT INTO offer_flights (offer_id) VALUES ($1) RETURNING id`

// CreateOfferFlight inserts a flight row for offerID and returns its id.
func (q *Queries) CreateOfferFlight(ctx context.Context, offerID int32) (int32, error) {
	var id int32
	err := q.db.QueryRow(ctx, createOfferFlightSQL, offerID).Scan(&id)
	return id, err
}

const listOfferFlightsByOfferIDSQL = `SELECT id, offer_id FROM offer_flights WHERE offer_id = $1 ORDER BY id ASC`

// ListOfferFlightsByOfferID returns the offer's flight rows, ordered by
// ascending id (i.e. insertion order).
func (q *Queries) ListOfferFlightsByOfferID(ctx context.Context, offerID int32) ([]OfferFlight, error) {
	rows, err := q.db.Query(ctx, listOfferFlightsByOfferIDSQL, offerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []OfferFlight
	for rows.Next() {
		var f OfferFlight
		if err := rows.Scan(&f.ID, &f.OfferID); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

const deleteOfferFlightsByOfferIDSQL = `DELETE FROM offer_flights WHERE offer_id = $1`

// DeleteOfferFlightsByOfferID removes every flight (and, via
// ON DELETE CASCADE, every segment) belonging to offerID.
func (q *Queries) DeleteOfferFlightsByOfferID(ctx context.Context, offerID int32) error {
	_, err := q.db.Exec(ctx, deleteOfferFlightsByOfferIDSQL, offerID)
	return err
}

const createOfferFlightSegmentSQL = `INSERT INTO offer_flight_segments (
    flight_id, sequence, departure_airport_icao, arrival_airport_icao, departure_at, arrival_at
) VALUES ($1, $2, $3, $4, $5, $6)`

// CreateOfferFlightSegmentParams matches the column order of
// createOfferFlightSegmentSQL.
type CreateOfferFlightSegmentParams struct {
	FlightID             int32
	Sequence             int16
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	DepartureAt          time.Time
	ArrivalAt            time.Time
}

// CreateOfferFlightSegment inserts one segment row.
func (q *Queries) CreateOfferFlightSegment(ctx context.Context, arg CreateOfferFlightSegmentParams) error {
	_, err := q.db.Exec(ctx, createOfferFlightSegmentSQL,
		arg.FlightID,
		arg.Sequence,
		arg.DepartureAirportICAO,
		arg.ArrivalAirportICAO,
		arg.DepartureAt,
		arg.ArrivalAt,
	)
	return err
}

// listOfferFlightSegmentsByFlightIDsSQL orders by flight then sequence
// so callers can group consecutive rows by flight_id without a map.
const listOfferFlightSegmentsByFlightIDsSQL = `SELECT id, flight_id, sequence, departure_airport_icao, arrival_airport_icao, departure_at, arrival_at
FROM offer_flight_segments
WHERE flight_id = ANY($1::int[])
ORDER BY flight_id ASC, sequence ASC`

// ListOfferFlightSegmentsByFlightIDs returns every segment belonging to
// any of flightIDs.
func (q *Queries) ListOfferFlightSegmentsByFlightIDs(ctx context.Context, flightIDs []int32) ([]OfferFlightSegment, error) {
	rows, err := q.db.Query(ctx, listOfferFlightSegmentsByFlightIDsSQL, flightIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []OfferFlightSegment
	for rows.Next() {
		var s OfferFlightSegment
		if err := rows.Scan(
			&s.ID, &s.FlightID, &s.Sequence,
			&s.DepartureAirportICAO, &s.ArrivalAirportICAO,
			&s.DepartureAt, &s.ArrivalAt,
		); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
