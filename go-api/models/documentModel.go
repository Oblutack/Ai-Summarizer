package models

import "gorm.io/gorm"

type Document struct {
	gorm.Model
	Filename string
	Summary  string `gorm:"type:text"` 
	UserID   uint   
}