package shared

import "time"

type EntriesListResponse struct {
	Entries    []Entry   `json:"entries"`
	ServerTime time.Time `json:"server_time"`
}
