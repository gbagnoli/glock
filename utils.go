package glock

import "github.com/nu7hatch/gouuid"

// UUID returns a new V4 UUID string
func UUID() (string, error) {
	u, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
