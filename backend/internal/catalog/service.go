package catalog

import "context"

type Service struct{ repository *Repository }

func NewService(repository *Repository) *Service { return &Service{repository: repository} }

func (s *Service) ListEvents(ctx context.Context) ([]Event, error) {
	return s.repository.ListEvents(ctx)
}

func (s *Service) GetEvent(ctx context.Context, id string) (Event, error) {
	return s.repository.GetEvent(ctx, id)
}
