package cart

import (
	"time"
)

// Item represents an item in a cart.
type Item struct {
	ID        int64     `json:"id"`
	CartID    string    `json:"cart_id"`
	VariantID int64     `json:"variant_id"`
	Quantity  int32     `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
}

// CartResponse represents the state of a shopping cart.
type CartResponse struct {
	ID        string `json:"id"`
	UserID    *int64 `json:"user_id,omitempty"`
	Items     []Item `json:"items"`
	ItemCount int32  `json:"item_count"`
}
