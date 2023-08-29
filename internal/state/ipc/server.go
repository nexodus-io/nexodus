package ipc

import "github.com/nexodus-io/nexodus/internal/state"

type Server struct {
	store     state.Store
	initError error
}

func NewServer(store state.Store, initError error) *Server {
	return &Server{store: store, initError: initError}
}

func (s *Server) Load(args bool, result *state.State) error {
	if s.initError != nil {
		return s.initError
	}
	err := s.store.Load()
	if err != nil {
		return err
	}
	*result = *s.store.State()
	return nil
}

func (s *Server) Store(args state.State, result *bool) error {
	if s.initError != nil {
		return s.initError
	}
	*s.store.State() = args
	return s.store.Store()
}
