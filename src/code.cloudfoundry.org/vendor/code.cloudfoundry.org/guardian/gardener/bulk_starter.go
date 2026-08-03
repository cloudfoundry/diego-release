package gardener

type bulkStarter struct {
	Starters []Starter
}

func NewBulkStarter(starters []Starter) *bulkStarter {
	return &bulkStarter{
		Starters: starters,
	}
}

func (b *bulkStarter) StartAll() error {
	for _, s := range b.Starters {
		if err := s.Start(); err != nil {
			return err
		}
	}
	return nil
}
