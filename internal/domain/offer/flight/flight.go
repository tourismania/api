// Package flight is the Flight child aggregate of an Offer: the Flight
// entity with its Segments and Layovers, the structural factory (New),
// the repository contract and the replace-set manager. It lives as a
// subpackage of domain/offer because a flight never exists outside an
// offer (every contract is scoped by offerID); future offer children
// (hotels, transfers, rents) follow the same layout — one subpackage
// per child aggregate, so the parent offer package stays the aggregate
// root only. Children may import the parent package; the parent never
// imports its children — composition happens in the application layer.
package flight

import "time"

// Segment is one nonstop leg of a Flight.
type Segment struct {
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	DepartureAt          time.Time
	ArrivalAt            time.Time
}

// Duration is the segment's own flight time.
func (s Segment) Duration() time.Duration {
	return s.ArrivalAt.Sub(s.DepartureAt)
}

// Layover is the wait between the arrival of one segment and the
// departure of the next.
type Layover struct {
	AirportICAO string
	Duration    time.Duration
}

// Flight is a journey from its first segment's departure airport to its
// last segment's arrival airport, made up of one or more nonstop
// Segments — 2+ segments mean one or more layovers. Structural
// invariants (non-empty Segments, chronology, route continuity, strictly
// positive layovers) are enforced by New, never here: this
// type only ever carries already-valid data. TotalDuration and Layovers
// are always computed on the fly from Segments, never stored.
type Flight struct {
	ID       int
	OfferID  int
	Segments []Segment
}

// DepartureAirportICAO is the airport of the flight's first segment.
func (f Flight) DepartureAirportICAO() string {
	return f.Segments[0].DepartureAirportICAO
}

// ArrivalAirportICAO is the airport of the flight's last segment.
func (f Flight) ArrivalAirportICAO() string {
	return f.Segments[len(f.Segments)-1].ArrivalAirportICAO
}

// TotalDuration is the elapsed time from the first segment's departure
// to the last segment's arrival, including every layover in between.
func (f Flight) TotalDuration() time.Duration {
	first := f.Segments[0]
	last := f.Segments[len(f.Segments)-1]
	return last.ArrivalAt.Sub(first.DepartureAt)
}

// Layovers returns one entry per stop between consecutive segments, in
// order. A nonstop flight (single segment) returns nil.
func (f Flight) Layovers() []Layover {
	if len(f.Segments) < 2 {
		return nil
	}
	layovers := make([]Layover, 0, len(f.Segments)-1)
	for i := 1; i < len(f.Segments); i++ {
		prev, next := f.Segments[i-1], f.Segments[i]
		layovers = append(layovers, Layover{
			AirportICAO: prev.ArrivalAirportICAO,
			Duration:    next.DepartureAt.Sub(prev.ArrivalAt),
		})
	}
	return layovers
}
