package product

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"waap-backend/pkg/response"
)

// Service defines business logic for the product catalog.
type Service interface {
	GetProductByID(ctx context.Context, id int64) (*ProductResponse, error)
	ListCategories(ctx context.Context) ([]CategoryResponse, error)
}

type service struct {
	repo Repository
}

// NewService constructs a product catalog service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetProductByID(ctx context.Context, id int64) (*ProductResponse, error) {
	p, err := s.repo.GetProductByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, response.NewNotFoundError("product not found")
		}
		return nil, response.NewInternalError(err)
	}

	return &ProductResponse{
		ID:          p.ID,
		CategoryID:  p.CategoryID,
		Title:       p.Title,
		Slug:        p.Slug,
		Description: p.Description,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt.Time,
		UpdatedAt:   p.UpdatedAt.Time,
	}, nil
}

func (s *service) ListCategories(ctx context.Context) ([]CategoryResponse, error) {
	cats, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, response.NewInternalError(err)
	}

	res := make([]CategoryResponse, 0, len(cats))
	for _, c := range cats {
		var parentID *int64
		if c.ParentID.Valid {
			pid := c.ParentID.Int64
			parentID = &pid
		}
		var desc *string
		if c.Description.Valid {
			d := c.Description.String
			desc = &d
		}
		var img *string
		if c.ImageUrl.Valid {
			u := c.ImageUrl.String
			img = &u
		}
		res = append(res, CategoryResponse{
			ID:          c.ID,
			ParentID:    parentID,
			Name:        c.Name,
			Slug:        c.Slug,
			Description: desc,
			ImageUrl:    img,
		})
	}
	return res, nil
}

