-- name: GetProductByID :one
SELECT * FROM products
WHERE id = $1 LIMIT 1;

-- name: GetProductBySlug :one
SELECT * FROM products
WHERE slug = $1 LIMIT 1;

-- name: DeductStockForCheckout :execrows
UPDATE product_variants
SET stock_quantity = stock_quantity - $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND stock_quantity >= $2;

