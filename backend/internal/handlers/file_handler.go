package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"stock-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const defaultUploadDir = "./uploads"

// uploadDir 回傳存放目錄；優先讀 UPLOAD_DIR 環境變數
func uploadDir() string {
	if d := os.Getenv("UPLOAD_DIR"); d != "" {
		return d
	}
	return defaultUploadDir
}

type FileHandler struct {
	db *gorm.DB
}

func NewFileHandler(db *gorm.DB) *FileHandler {
	return &FileHandler{db: db}
}

// List GET /api/files
func (h *FileHandler) List(c *gin.Context) {
	var files []models.UploadedFile
	if err := h.db.Order("uploaded_at desc").Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查詢失敗"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": files})
}

// Upload POST /api/files
// multipart/form-data  field: file
func (h *FileHandler) Upload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請選擇要上傳的檔案"})
		return
	}

	// 確保存放目錄存在
	dir := uploadDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "無法建立儲存目錄"})
		return
	}

	// 產生唯一存儲檔名（timestamp + original）
	ext := filepath.Ext(fh.Filename)
	storedName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), sanitizeFilename(fh.Filename))
	destPath := filepath.Join(dir, storedName)

	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "無法開啟上傳檔案"})
		return
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "無法儲存檔案"})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "寫入檔案失敗"})
		return
	}

	// 猜測 content type
	contentType := fh.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mimeByExt(ext)
	}

	record := models.UploadedFile{
		OriginalName: fh.Filename,
		StoredName:   storedName,
		Size:         written,
		ContentType:  contentType,
	}
	if err := h.db.Create(&record).Error; err != nil {
		// rollback 檔案
		_ = os.Remove(destPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "儲存記錄失敗"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": record})
}

// Download GET /api/files/:id/download
func (h *FileHandler) Download(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "無效的檔案 ID"})
		return
	}

	var record models.UploadedFile
	if err := h.db.First(&record, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "找不到該檔案"})
		return
	}

	filePath := filepath.Join(uploadDir(), record.StoredName)
	// 確認路徑在 uploadDir 內，防止路徑遍歷攻擊
	absUploadDir, _ := filepath.Abs(uploadDir())
	absFilePath, _ := filepath.Abs(filePath)
	if !strings.HasPrefix(absFilePath, absUploadDir+string(os.PathSeparator)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "存取被拒絕"})
		return
	}

	if _, err := os.Stat(absFilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "檔案不存在於伺服器"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, record.OriginalName))
	c.Header("Content-Type", record.ContentType)
	c.File(absFilePath)
}

// Delete DELETE /api/files/:id
func (h *FileHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "無效的檔案 ID"})
		return
	}

	var record models.UploadedFile
	if err := h.db.First(&record, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "找不到該檔案"})
		return
	}

	filePath := filepath.Join(uploadDir(), record.StoredName)
	absUploadDir, _ := filepath.Abs(uploadDir())
	absFilePath, _ := filepath.Abs(filePath)
	if !strings.HasPrefix(absFilePath, absUploadDir+string(os.PathSeparator)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "存取被拒絕"})
		return
	}

	_ = os.Remove(absFilePath)

	if err := h.db.Delete(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "刪除記錄失敗"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// sanitizeFilename 移除路徑分隔符與危險字元，只保留最後的檔名部分
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	// 把空白與非 ASCII 以外的危險符號替換為 _
	var sb strings.Builder
	for _, r := range name {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			sb.WriteRune('_')
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func mimeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".zip":
		return "application/zip"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "application/octet-stream"
	}
}
