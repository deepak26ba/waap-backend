package product

import (
	"time"
)

// ProductResponse models the client representation of a product.
type ProductResponse struct {
	ID          int64     `json:"id"`
	CategoryID  int64     `json:"category_id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	BasePrice   string    `json:"base_price"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CategoryResponse models category data.
type CategoryResponse struct {
	ID          int64   `json:"id"`
	ParentID    *int64  `json:"parent_id,omitempty"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
	ImageUrl    *string `json:"image_url,omitempty"`
}

