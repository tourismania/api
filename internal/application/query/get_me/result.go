package getme

import (
	"api/internal/domain/valueobject"

	"github.com/google/uuid"
)

// Agency is the minimal agency projection attached to a user's profile.
type Agency struct {
	ID   int
	UUID uuid.UUID
	Name string
}

// Result is the application-layer view returned to the presentation layer.
// Rights is a domain Value Object; the HTTP layer is responsible for
// projecting it into a transport DTO.
type Result struct {
	Uuid      uuid.UUID `json:"uuid"`
	Email     string
	Phone     string
	FirstName string
	LastName  string
	Rights    valueobject.RightsDescribe
	Agency    Agency
}
