package shared

import "time"

const (
	FriendshipStatusPending  = "pending"
	FriendshipStatusAccepted = "accepted"
	FriendshipStatusDeclined = "declined"
)

type Friendship struct {
	ID          string     `json:"id"`
	RequestedBy string     `json:"requested_by"`
	AcceptedBy  string     `json:"accepted_by"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	AcceptedAt  *time.Time `json:"accepted_at"`
	DeclinedAt  *time.Time `json:"declined_at"`
}

type FriendshipsListResponse struct {
	Friendships []Friendship `json:"friendships"`
}

type FriendshipCreateRequest struct {
	Username string `json:"username"`
}
