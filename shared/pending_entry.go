package shared

import "time"

type PendingEntry struct {
	ID            string    `json:"id"`
	From          string    `json:"from"`
	SourceEntryID string    `json:"source_entry_id"`
	Title         string    `json:"title"`
	URL           string    `json:"url"`
	Note          string    `json:"note"`
	Tags          []string  `json:"tags"`
	SharedAt      time.Time `json:"shared_at"`
}

type PendingEntriesListResponse struct {
	PendingEntries []PendingEntry `json:"pending_entries"`
}

type ShareEntryRequest struct {
	Username string `json:"username"`
}
