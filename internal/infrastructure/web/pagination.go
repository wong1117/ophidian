package web

import (
	"strconv"

	"github.com/labstack/echo/v4"
)

type Pagination struct {
	Page    int
	PerPage int
	Offset  int
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination PageMeta    `json:"pagination"`
}

type PageMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func PaginationFromContext(c echo.Context) Pagination {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.QueryParam("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return Pagination{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}
}

func Paginated[T any](data []T, total int, p Pagination) PaginatedResponse {
	totalPages := total / p.PerPage
	if total%p.PerPage > 0 {
		totalPages++
	}
	return PaginatedResponse{
		Data: data,
		Pagination: PageMeta{
			Page:       p.Page,
			PerPage:    p.PerPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

func FilterFromContext(c echo.Context) map[string]string {
	filters := make(map[string]string)
	for key, values := range c.QueryParams() {
		if key != "page" && key != "per_page" && len(values) > 0 {
			filters[key] = values[0]
		}
	}
	return filters
}
