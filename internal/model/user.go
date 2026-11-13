package model

import "time"

// UserResponse represents safe, serialized user profile data.
type UserResponse struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	FullName    string    `json:"full_name"`
	Role        string    `json:"role"`
	PhoneNumber *string   `json:"phone_number,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// RegisterInput models the registration request payload.
type RegisterInput struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	FullName    string  `json:"full_name"`
	PhoneNumber *string `json:"phone_number,omitempty"`
}

// LoginInput models the login request credentials.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse contains authentication tokens and the authenticated user's profile.
type LoginResponse struct {
	TokenPair
	User UserResponse `json:"user"`
}
