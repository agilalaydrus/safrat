package domain

import "time"

type PilgrimRegistration struct {
	ID             string
	OperatorID     string
	SeasonID       string
	ProductID      string
	FullName       string
	PassportNumber string
	DateOfBirth    *time.Time
	Gender         string
	Phone          string
	Email          string
	Nationality    string
	Address        string
	Status         string
	Notes          string
	CreatedAt      time.Time
}
