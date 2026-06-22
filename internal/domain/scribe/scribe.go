package scribe

import (
	"errors"
	"strings"
	"time"
)

type Scribe struct {
	ID         string     `json:"id"`
	OwnerID    string     `json:"owner_id"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	IsPinned   bool       `json:"is_pinned"`
	IsArchived bool       `json:"is_archived"`
	Tags       []string   `json:"tags"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (n *Scribe) Validate() error {
	if strings.TrimSpace(n.OwnerID) == "" {
		return errors.New("scribe owner id cannot be empty")
	}
	if strings.TrimSpace(n.Title) == "" {
		return errors.New("scribe title cannot be empty")
	}
	if len(n.Title) > 255 {
		return errors.New("scribe title cannot exceed 255 characters")
	}
	return nil
}

func (n *Scribe) Pin() {
	n.IsPinned = true
}

func (n *Scribe) Unpin() {
	n.IsPinned = false
}

func (n *Scribe) Archive() error {
	if n.IsArchived {
		return errors.New("scribe is already archived")
	}
	n.IsArchived = true
	n.IsPinned = false
	return nil
}

func (n *Scribe) Restore() error {
	if !n.IsArchived && n.DeletedAt == nil {
		return errors.New("scribe is not archived or deleted")
	}
	n.IsArchived = false
	n.DeletedAt = nil
	return nil
}
