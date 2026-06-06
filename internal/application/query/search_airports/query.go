package searchairports

// Query is the input for the SearchAirports use-case.
type Query struct {
	Search string
	Limit  int
	Offset int
}
