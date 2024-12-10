package data

import (
	"github.com/LTXWorld/greenLight_copy/internal/validator"
	"math"
	"strings"
)

type Filters struct {
	Page         int
	PageSize     int
	Sort         string // 按什么排序，-代表降序（year）
	SortSafelist []string
}

// Check the client-provided Sort field matches one of the entries in our safelist
// and if it does, extract the column name from the Sort field by stripping the leading hyphen character
func (f Filters) sortColumn() string {
	for _, safeValue := range f.SortSafelist {
		if f.Sort == safeValue {
			return strings.TrimPrefix(f.Sort, "-") // 如果不以-开头，就返回原来的Sort
		}
	}

	// 按道理来说sort不应该不存在，因为前面做过Validate验证，所以如果真的不存在，需要在这里panic防止SQL注入。
	panic("unsafe sort parameter:" + f.Sort)
}

// Return the sort direction (ASC or DESC) depending on the prefix
func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}

	return "ASC"
}

func ValidateFilters(v *validator.Validator, f Filters) {
	// Check that the page and page_size parameters contain sensible values.
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a maximum of 10 million")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")
	// Check that the sort parameter matches a value in the safelist.
	v.Check(validator.In(f.Sort, f.SortSafelist...), "sort", "invalid sort value")
}

func (f Filters) limit() int {
	return f.PageSize
}

func (f Filters) offset() int {
	return (f.Page - 1) * f.PageSize // 由于在ValidateFilters中已经设置了page_size和page的最大值
}

// Define a new Metadata struct for holding the pagination metadata
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

// 根据记录总数，当前页码和每页大小的值计算适当的分页元数据值（结构体中其他值）
func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}

	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
	}
}
