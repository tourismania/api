package user

import "strconv"

// Registered fires after a user is persisted. The body is intentionally
// minimal — downstream services should rehydrate from the DB by ID.
type Registered struct {
	ID int `json:"id"`
}

const registeredCode = "user_registered"

// GetKey is used as the Kafka partition key.
func (e Registered) GetKey() string { return strconv.Itoa(e.ID) }

// GetEventCode is the discriminator written into the message body.
func (e Registered) GetEventCode() string { return registeredCode }
