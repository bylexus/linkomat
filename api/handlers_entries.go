package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bylexus/linkomat/shared"
	"gorm.io/gorm"
)

const (
	defaultEntriesLimit = 200
	maxEntriesLimit     = 1000
	maxEntryBodyBytes   = 64 * 1024
	maxEntryIDLen       = 64
)

func (s *Server) handleEntriesList(w http.ResponseWriter, r *http.Request) {
	username := usernameFromCtx(r.Context())
	udb, err := s.userDBs.For(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open user db.", err.Error())
		return
	}

	serverTime := time.Now().UTC()

	q := r.URL.Query()
	if cs := q.Get("changed_since"); cs != "" {
		t, err := time.Parse(time.RFC3339, cs)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid-request", "changed_since must be RFC3339.", err.Error())
			return
		}
		var rows []Entry
		if err := udb.Where("updated_at > ?", t).Order("updated_at asc, id asc").Find(&rows).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Could not list entries.", err.Error())
			return
		}
		writeEntriesList(w, rows, serverTime)
		return
	}

	limit := defaultEntriesLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid-request", "limit must be a positive integer.", "")
			return
		}
		if n > maxEntriesLimit {
			n = maxEntriesLimit
		}
		limit = n
	}
	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid-request", "offset must be a non-negative integer.", "")
			return
		}
		offset = n
	}

	var rows []Entry
	if err := udb.Order("updated_at asc, id asc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not list entries.", err.Error())
		return
	}
	writeEntriesList(w, rows, serverTime)
}

func (s *Server) handleEntryGet(w http.ResponseWriter, r *http.Request) {
	username := usernameFromCtx(r.Context())
	udb, err := s.userDBs.For(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open user db.", err.Error())
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid-request", "Entry id is required.", "")
		return
	}
	var e Entry
	if err := udb.Where("id = ?", id).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "not-found", "Entry not found.", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up entry.", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entryToDTO(&e))
}

func (s *Server) handleEntryPut(w http.ResponseWriter, r *http.Request) {
	username := usernameFromCtx(r.Context())
	udb, err := s.userDBs.For(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open user db.", err.Error())
		return
	}
	id := r.PathValue("id")
	if id == "" || len(id) > maxEntryIDLen {
		writeError(w, http.StatusBadRequest, "invalid-request", "Entry id is required and must be ≤ 64 chars.", "")
		return
	}
	var payload shared.Entry
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxEntryBodyBytes)).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", "Could not parse request body.", err.Error())
		return
	}
	if payload.ID != "" && payload.ID != id {
		writeError(w, http.StatusBadRequest, "invalid-request", "Body id does not match URL.", "")
		return
	}

	now := time.Now().UTC()
	var existing Entry
	err = udb.Where("id = ?", id).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up entry.", err.Error())
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		existing = Entry{
			ID:        id,
			Title:     payload.Title,
			URL:       payload.URL,
			Note:      payload.Note,
			Tags:      encodeTags(payload.Tags),
			Meta:      "{}",
			CreatedAt: now,
			UpdatedAt: now,
			DeletedAt: nil,
		}
		if err := udb.Create(&existing).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Could not create entry.", err.Error())
			return
		}
	} else {
		existing.Title = payload.Title
		existing.URL = payload.URL
		existing.Note = payload.Note
		existing.Tags = encodeTags(payload.Tags)
		existing.UpdatedAt = now
		existing.DeletedAt = nil
		if err := udb.Save(&existing).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Could not update entry.", err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, entryToDTO(&existing))
}

func (s *Server) handleEntryDelete(w http.ResponseWriter, r *http.Request) {
	username := usernameFromCtx(r.Context())
	udb, err := s.userDBs.For(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open user db.", err.Error())
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid-request", "Entry id is required.", "")
		return
	}
	var e Entry
	err = udb.Where("id = ?", id).First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "not-found", "Entry not found.", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up entry.", err.Error())
		return
	}
	now := time.Now().UTC()
	e.DeletedAt = &now
	e.UpdatedAt = now
	if err := udb.Save(&e).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not delete entry.", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeEntriesList(w http.ResponseWriter, rows []Entry, serverTime time.Time) {
	entries := make([]shared.Entry, 0, len(rows))
	for i := range rows {
		entries = append(entries, entryToDTO(&rows[i]))
	}
	writeJSON(w, http.StatusOK, shared.EntriesListResponse{
		Entries:    entries,
		ServerTime: serverTime,
	})
}

func entryToDTO(e *Entry) shared.Entry {
	return shared.Entry{
		ID:        e.ID,
		Title:     e.Title,
		URL:       e.URL,
		Note:      e.Note,
		Tags:      decodeTags(e.Tags),
		Meta:      metaToRaw(e.Meta),
		CreatedAt: e.CreatedAt.UTC(),
		UpdatedAt: e.UpdatedAt.UTC(),
		DeletedAt: utcPtr(e.DeletedAt),
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func encodeTags(tags []string) string {
	if tags == nil {
		return "[]"
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decodeTags(s string) []string {
	if s == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(s), &tags); err != nil {
		return []string{}
	}
	if tags == nil {
		return []string{}
	}
	return tags
}
