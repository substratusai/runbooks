package utils

import "github.com/google/uuid"

var NewUUID = func() string {
	return uuid.New().String()
}
