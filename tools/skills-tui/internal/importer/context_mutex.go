package importer

import "context"

type contextMutex struct {
	token chan struct{}
}

func newContextMutex() *contextMutex {
	mutex := &contextMutex{token: make(chan struct{}, 1)}
	mutex.token <- struct{}{}
	return mutex
}

func (m *contextMutex) lock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.token:
		return nil
	}
}

func (m *contextMutex) unlock() {
	m.token <- struct{}{}
}
