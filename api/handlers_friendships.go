package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bylexus/linkomat/shared"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxFriendshipBodyBytes = 1024

type friendshipError struct {
	status  int
	errType string
	title   string
}

func (e *friendshipError) Error() string {
	return e.title
}

func newUUIDv4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func (s *Server) handleFriendshipsList(w http.ResponseWriter, r *http.Request) {
	uid := userIDValueFromCtx(r.Context())
	var rows []Friendship
	err := s.meta.Preload("RequestedBy").Preload("AcceptedBy").
		Where("requested_by_id = ? OR (accepted_by_id = ? AND declined_at IS NULL)", uid, uid).
		Order("requested_at desc").
		Find(&rows).Error
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not list friendships.", err.Error())
		return
	}
	list := make([]shared.Friendship, 0, len(rows))
	for i := range rows {
		list = append(list, friendshipToDTO(&rows[i]))
	}
	writeJSON(w, http.StatusOK, shared.FriendshipsListResponse{Friendships: list})
}

func (s *Server) handleFriendshipCreate(w http.ResponseWriter, r *http.Request) {
	uid := userIDValueFromCtx(r.Context())
	var req shared.FriendshipCreateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFriendshipBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", "Could not parse request body.", err.Error())
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		writeError(w, http.StatusBadRequest, "invalid-request", "Username is required.", "")
		return
	}
	if username == usernameFromCtx(r.Context()) {
		writeError(w, http.StatusBadRequest, "invalid-request", "You cannot befriend yourself.", "")
		return
	}

	var result Friendship
	status := http.StatusCreated
	err := s.meta.Transaction(func(tx *gorm.DB) error {
		var target User
		err := tx.Where("username = ?", username).First(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !target.Active) {
			return &friendshipError{http.StatusNotFound, "not-found", "User not found."}
		}
		if err != nil {
			return err
		}

		var existing Friendship
		err = tx.Where("requested_by_id = ? AND accepted_by_id = ?", uid, target.ID).First(&existing).Error
		if err == nil {
			return &friendshipError{http.StatusConflict, "conflict", "Friend request already exists."}
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := time.Now().UTC()
		var reverse Friendship
		err = tx.Where("requested_by_id = ? AND accepted_by_id = ?", target.ID, uid).First(&reverse).Error
		if err == nil {
			if reverse.AcceptedAt != nil && reverse.DeclinedAt == nil {
				return &friendshipError{http.StatusConflict, "conflict", "Already friends."}
			}
			if err := tx.Model(&reverse).Updates(map[string]any{"accepted_at": now, "declined_at": nil}).Error; err != nil {
				return err
			}
			status = http.StatusOK
			return tx.Preload("RequestedBy").Preload("AcceptedBy").Where("id = ?", reverse.ID).First(&result).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		id, err := newUUIDv4()
		if err != nil {
			return err
		}
		f := Friendship{
			ID:            id,
			RequestedByID: uid,
			AcceptedByID:  target.ID,
			RequestedAt:   now,
		}
		if err := tx.Omit(clause.Associations).Create(&f).Error; err != nil {
			return err
		}
		return tx.Preload("RequestedBy").Preload("AcceptedBy").Where("id = ?", f.ID).First(&result).Error
	})
	if err != nil {
		writeFriendshipError(w, err, "Could not create friendship.")
		return
	}
	writeJSON(w, status, friendshipToDTO(&result))
}

func (s *Server) handleFriendshipAccept(w http.ResponseWriter, r *http.Request) {
	s.answerFriendship(w, r, "accepted_at", "Could not accept friendship.")
}

func (s *Server) handleFriendshipDecline(w http.ResponseWriter, r *http.Request) {
	s.answerFriendship(w, r, "declined_at", "Could not decline friendship.")
}

func (s *Server) answerFriendship(w http.ResponseWriter, r *http.Request, column, failTitle string) {
	uid := userIDValueFromCtx(r.Context())
	id := r.PathValue("id")
	var result Friendship
	err := s.meta.Transaction(func(tx *gorm.DB) error {
		var f Friendship
		err := tx.Where("id = ? AND (requested_by_id = ? OR accepted_by_id = ?)", id, uid, uid).First(&f).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &friendshipError{http.StatusNotFound, "not-found", "Friendship not found."}
		}
		if err != nil {
			return err
		}
		if f.AcceptedByID != uid {
			return &friendshipError{http.StatusForbidden, "forbidden", "Only the addressee can answer a friend request."}
		}
		if f.AcceptedAt != nil || f.DeclinedAt != nil {
			return &friendshipError{http.StatusConflict, "conflict", "Friend request is not pending."}
		}
		if err := tx.Model(&f).Update(column, time.Now().UTC()).Error; err != nil {
			return err
		}
		return tx.Preload("RequestedBy").Preload("AcceptedBy").Where("id = ?", f.ID).First(&result).Error
	})
	if err != nil {
		writeFriendshipError(w, err, failTitle)
		return
	}
	writeJSON(w, http.StatusOK, friendshipToDTO(&result))
}

func (s *Server) handleFriendshipDelete(w http.ResponseWriter, r *http.Request) {
	uid := userIDValueFromCtx(r.Context())
	id := r.PathValue("id")
	res := s.meta.Where("id = ? AND (requested_by_id = ? OR accepted_by_id = ?)", id, uid, uid).Delete(&Friendship{})
	if res.Error != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not delete friendship.", res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "not-found", "Friendship not found.", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeFriendshipError(w http.ResponseWriter, err error, failTitle string) {
	var fe *friendshipError
	if errors.As(err, &fe) {
		writeError(w, fe.status, fe.errType, fe.title, "")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", failTitle, err.Error())
}

func friendshipStatus(f *Friendship) string {
	if f.DeclinedAt != nil {
		return shared.FriendshipStatusDeclined
	}
	if f.AcceptedAt != nil {
		return shared.FriendshipStatusAccepted
	}
	return shared.FriendshipStatusPending
}

func friendshipToDTO(f *Friendship) shared.Friendship {
	return shared.Friendship{
		ID:          f.ID,
		RequestedBy: f.RequestedBy.Username,
		AcceptedBy:  f.AcceptedBy.Username,
		Status:      friendshipStatus(f),
		RequestedAt: f.RequestedAt.UTC(),
		AcceptedAt:  utcPtr(f.AcceptedAt),
		DeclinedAt:  utcPtr(f.DeclinedAt),
	}
}
