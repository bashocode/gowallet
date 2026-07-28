package model

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PaginationParams struct {
	Page   int    `form:"page,default=1"`
	Limit  int    `form:"limit,default=10"`
	Sort   string `form:"sort,default=created_at"`
	Order  string `form:"order,default=desc"`
	Status string `form:"status"`  // optional filter status
	Cursor string `form:"cursor"` // cursor-based pagination token
}

func (p *PaginationParams) Offset() int {
	return (p.Page - 1) * p.Limit
}

func (p *PaginationParams) HasCursor() bool {
	return p.Cursor != ""
}

type PaginationMeta struct {
	Page       int     `json:"page"`
	Limit      int     `json:"limit"`
	Total      int64   `json:"total"`
	TotalPage  int     `json:"total_page"`
	NextCursor *string `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
}

type PaginatedResponse struct {
	Success bool           `json:"success"`
	Data    any            `json:"data"`
	Meta    PaginationMeta `json:"meta"`
}

func EncodeCursor(t time.Time, id string) string {
	data := fmt.Sprintf("%d,%s", t.UnixMilli(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(data))
}

func DecodeCursor(cursor string) (time.Time, string, error) {
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(data), ",", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor format")
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", err
	}
	return time.UnixMilli(ms).UTC(), parts[1], nil
}
