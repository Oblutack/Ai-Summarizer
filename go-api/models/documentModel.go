package models

import "gorm.io/gorm"

type Document struct {
	gorm.Model
	Filename string
	Summary  string `gorm:"type:text"`
	UserID   uint

	// Content is the source text kept so the user can chat with the document. It is never
	// sent to clients; HasContent tells them whether chat is available.
	Content    string `gorm:"type:text" json:"-"`
	HasContent bool   `json:"hasContent"`
}
