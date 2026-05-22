package getme

import (
	"api/internal/domain/valueobject"

	"github.com/google/uuid"
)

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
}
