package models

import "time"

// UploadedFile 暫存上傳檔案的 metadata
type UploadedFile struct {
	ID           uint      `json:"id"            gorm:"primarykey;autoIncrement"`
	OriginalName string    `json:"original_name" gorm:"not null"`
	StoredName   string    `json:"stored_name"   gorm:"not null;uniqueIndex"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"  gorm:"default:''"`
	UploadedAt   time.Time `json:"uploaded_at"   gorm:"autoCreateTime"`
}
