package models

import "time"

// DocumentFile is an original (a PDF, or a recording) kept with a saved summary. Content is never part of a JSON
// response; clients get the list (FileInfo) and download bytes through a dedicated endpoint.
type DocumentFile struct {
	ID         uint `gorm:"primaryKey"`
	DocumentID uint
	UserID     uint
	Filename   string
	SizeBytes  int64
	Content    []byte `gorm:"type:bytea" json:"-"`
	CreatedAt  time.Time
}

// FileInfo describes a stored file to clients.
type FileInfo struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}
