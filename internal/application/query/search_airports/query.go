package searchairports

// Query is the input for the SearchAirports use-case.
type Query struct {
	Search  string
	Country *string // nil — no filter
	Limit   int
	Offset  int
}
