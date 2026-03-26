package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DBViewerHandler struct {
	db *gorm.DB
}

func NewDBViewerHandler(db *gorm.DB) *DBViewerHandler {
	return &DBViewerHandler{db: db}
}

// 只允許合法的資料表名稱，防止 SQL injection
var validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type tableRow struct {
	TableName string `gorm:"column:table_name" json:"name"`
}

func (h *DBViewerHandler) tableExists(name string) bool {
	var exists bool
	h.db.Raw(
		`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=?)`,
		name,
	).Scan(&exists)
	return exists
}

// ListTables GET /api/admin/db/tables
// 回傳所有 public schema 的資料表名稱及列數
func (h *DBViewerHandler) ListTables(c *gin.Context) {
	var rows []tableRow
	if err := h.db.Raw(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type TableInfo struct {
		Name     string `json:"name"`
		RowCount int64  `json:"row_count"`
	}

	result := make([]TableInfo, 0, len(rows))
	for _, r := range rows {
		var count int64
		// r.TableName 來自 information_schema，已通過 DB 驗證，安全可直接插入
		h.db.Raw(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, r.TableName)).Scan(&count)
		result = append(result, TableInfo{Name: r.TableName, RowCount: count})
	}

	c.JSON(http.StatusOK, result)
}

// TableColumns GET /api/admin/db/tables/:name/columns
// 回傳指定資料表的欄位資訊
func (h *DBViewerHandler) TableColumns(c *gin.Context) {
	name := c.Param("name")
	if !validTableName.MatchString(name) || !h.tableExists(name) {
		c.JSON(http.StatusNotFound, gin.H{"error": "table not found"})
		return
	}

	type Column struct {
		Name     string `gorm:"column:column_name" json:"name"`
		DataType string `gorm:"column:data_type" json:"type"`
		Nullable string `gorm:"column:is_nullable" json:"nullable"`
		Default  string `gorm:"column:col_default" json:"default"`
	}

	var cols []Column
	h.db.Raw(`
		SELECT
			column_name,
			data_type,
			is_nullable,
			COALESCE(column_default, '') AS col_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?
		ORDER BY ordinal_position
	`, name).Scan(&cols)

	c.JSON(http.StatusOK, cols)
}

// TableData GET /api/admin/db/tables/:name/data?page=1&limit=50
// 回傳指定資料表的資料（可分頁）
func (h *DBViewerHandler) TableData(c *gin.Context) {
	name := c.Param("name")
	if !validTableName.MatchString(name) || !h.tableExists(name) {
		c.JSON(http.StatusNotFound, gin.H{"error": "table not found"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	var total int64
	h.db.Raw(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, name)).Scan(&total)

	var data []map[string]interface{}
	h.db.Raw(fmt.Sprintf(`SELECT * FROM "%s" LIMIT ? OFFSET ?`, name), limit, offset).Scan(&data)

	c.JSON(http.StatusOK, gin.H{
		"data":  data,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": (total + int64(limit) - 1) / int64(limit),
	})
}
