package pagination

// Pagination standardizes limit and offset based constraints for list queries.
// PageNumber is 0-based; PageSize defaults to the value passed to Normalize when zero.
type Pagination struct {
	PageSize   int
	PageNumber int
}

// Normalize applies sensible defaults: pageSize defaults to defaultPageSize,
// and pageNumber is clamped to >= 0.
func (p *Pagination) Normalize(defaultPageSize int) {
	if p.PageSize <= 0 {
		p.PageSize = defaultPageSize
	}
	if p.PageNumber < 0 {
		p.PageNumber = 0
	}
}
