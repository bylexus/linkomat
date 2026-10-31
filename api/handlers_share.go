package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bylexus/linkomat/shared"
	"gorm.io/gorm"
)

const maxShareBodyBytes = 1024

type shareError struct {
	status  int
	errType string
	title   string
}

func (e *shareError) Error() string {
	return e.title
}

func (s *Server) handleEntryShare(w http.ResponseWriter, r *http.Request) {
	uid := userIDValueFromCtx(r.Context())
	me := usernameFromCtx(r.Context())
	var req shared.ShareEntryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxShareBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", "Could not parse request body.", err.Error())
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		writeError(w, http.StatusBadRequest, "invalid-request", "Username is required.", "")
		return
	}
	if username == me {
		writeError(w, http.StatusBadRequest, "invalid-request", "You cannot share with yourself.", "")
		return
	}

	udb, err := s.userDBs.For(me)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open user db.", err.Error())
		return
	}
	var e Entry
	err = udb.Where("id = ? AND deleted_at IS NULL", r.PathValue("id")).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusNotFound, "not-found", "Entry not found.", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up entry.", err.Error())
		return
	}

	var target User
	err = s.meta.Where("username = ?", username).First(&target).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !target.Active) {
		writeError(w, http.StatusNotFound, "not-found", "User not found.", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up user.", err.Error())
		return
	}

	var friends int64
	err = s.meta.Model(&Friendship{}).
		Where("accepted_at IS NOT NULL AND declined_at IS NULL AND ((requested_by_id = ? AND accepted_by_id = ?) OR (requested_by_id = ? AND accepted_by_id = ?))", uid, target.ID, target.ID, uid).
		Count(&friends).Error
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up friendship.", err.Error())
		return
	}
	if friends == 0 {
		writeError(w, http.StatusForbidden, "forbidden", "Not friends.", "")
		return
	}

	rdb, err := s.userDBs.For(target.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open receiver db.", err.Error())
		return
	}
	id, err := newUUIDv4()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not share entry.", err.Error())
		return
	}
	now := time.Now().UTC()
	p := PendingEntry{
		ID:            id,
		FromUsername:  me,
		SourceEntryID: e.ID,
		Title:         e.Title,
		URL:           e.URL,
		Note:          e.Note,
		Tags:          e.Tags,
		SharedAt:      now,
	}
	if err := rdb.Create(&p).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not share entry.", err.Error())
		return
	}

	m := readMeta(e.Meta)
	shares, _ := m["shares"].([]any)
	m["shares"] = append(shares, map[string]any{"to": target.Username, "at": now.Format(time.RFC3339)})
	e.Meta = encodeMeta(m)
	e.UpdatedAt = now
	if err := udb.Save(&e).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not update entry.", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entryToDTO(&e))
}

func (s *Server) handlePendingEntriesList(w http.ResponseWriter, r *http.Request) {
	udb, err := s.userDBs.For(usernameFromCtx(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open user db.", err.Error())
		return
	}
	var rows []PendingEntry
	if err := udb.Order("shared_at desc, id asc").Find(&rows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not list pending entries.", err.Error())
		return
	}
	list := make([]shared.PendingEntry, 0, len(rows))
	for i := range rows {
		list = append(list, pendingEntryToDTO(&rows[i]))
	}
	writeJSON(w, http.StatusOK, shared.PendingEntriesListResponse{PendingEntries: list})
}

func (s *Server) handlePendingEntryAccept(w http.ResponseWriter, r *http.Request) {
	udb, err := s.userDBs.For(usernameFromCtx(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open user db.", err.Error())
		return
	}
	id := r.PathValue("id")
	var result Entry
	err = udb.Transaction(func(tx *gorm.DB) error {
		var p PendingEntry
		err := tx.Where("id = ?", id).First(&p).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &shareError{http.StatusNotFound, "not-found", "Pending entry not found."}
		}
		if err != nil {
			return err
		}
		newID, err := newUUIDv4()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		result = Entry{
			ID:    newID,
			Title: p.Title,
			URL:   p.URL,
			Note:  p.Note,
			Tags:  p.Tags,
			Meta: encodeMeta(map[string]any{
				"imported": map[string]any{
					"from":      p.FromUsername,
					"shared_at": p.SharedAt.UTC().Format(time.RFC3339),
					"at":        now.Format(time.RFC3339),
				},
			}),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return tx.Delete(&p).Error
	})
	if err != nil {
		writeShareError(w, err, "Could not accept pending entry.")
		return
	}
	writeJSON(w, http.StatusOK, entryToDTO(&result))
}

func (s *Server) handlePendingEntryDecline(w http.ResponseWriter, r *http.Request) {
	udb, err := s.userDBs.For(usernameFromCtx(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not open user db.", err.Error())
		return
	}
	res := udb.Where("id = ?", r.PathValue("id")).Delete(&PendingEntry{})
	if res.Error != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not decline pending entry.", res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "not-found", "Pending entry not found.", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeShareError(w http.ResponseWriter, err error, failTitle string) {
	var se *shareError
	if errors.As(err, &se) {
		writeError(w, se.status, se.errType, se.title, "")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", failTitle, err.Error())
}

func pendingEntryToDTO(p *PendingEntry) shared.PendingEntry {
	return shared.PendingEntry{
		ID:            p.ID,
		From:          p.FromUsername,
		SourceEntryID: p.SourceEntryID,
		Title:         p.Title,
		URL:           p.URL,
		Note:          p.Note,
		Tags:          decodeTags(p.Tags),
		SharedAt:      p.SharedAt.UTC(),
	}
}

func readMeta(s string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func encodeMeta(m map[string]any) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func metaToRaw(s string) json.RawMessage {
	return json.RawMessage(encodeMeta(readMeta(s)))
}
