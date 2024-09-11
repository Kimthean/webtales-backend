package types

import "go-novel/models"

type SignupRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type GoogleOAuthRequest struct {
	Code string `json:"code"`
}

type GoogleOAuthResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

type UpdateProfileRequest struct {
	Username string `json:"username"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}
