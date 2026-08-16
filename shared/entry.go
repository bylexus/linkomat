package shared

import "time"

type Entry struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	Note      string     `json:"note"`
	Tags      []string   `json:"tags"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}