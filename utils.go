package glock

import "github.com/satori/go.uuid"

// UUID returns a new V4 UUID string
func UUID() string {
	return uuid.NewV4().String()
}
