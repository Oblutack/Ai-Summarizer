package models

import (
	"time"

	"gorm.io/gorm"
)

type Document struct {
	gorm.Model
	Filename string
	Summary  string `gorm:"type:text"`
	UserID   uint

	// Content is the source text kept so the user can chat with the document. It is never
	// sent to clients; HasContent tells them whether chat is available.
	Content    string `gorm:"type:text" json:"-"`
	HasContent bool   `json:"hasContent"`

	// IndexedAt is when the document was cut into searchable passages; nil until then.
	IndexedAt *time.Time `json:"-"`

	// Files are the original PDFs, when they were kept (documents saved earlier have none).
	// Filled in by the list endpoint; not a database column.
	Files []FileInfo `gorm:"-" json:"files"`
}
