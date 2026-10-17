// Package uid holds the one way the modules read an ID that came from a
// client.
package uid

import "uuid"

// Canonical returns id in canonical form, or false when it is not a UUID:
// such an ID names nothing, and is answered as "not found" without a trip to
// the database.
func Canonical(id string) (string, bool) {
	u, err := uuid.Parse(id)
	if err != nil {
		return "", false
	}
	return u.String(), true
}
