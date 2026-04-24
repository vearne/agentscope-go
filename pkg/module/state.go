package module

import "sync"

type StateModule struct {
	state map[string]interface{}
	mu    sync.RWMutex
}

func NewStateModule() *StateModule {
	return &StateModule{
		state: make(map[string]interface{}),
	}
}

func (s *StateModule) SetState(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[key] = value
}

func (s *StateModule) GetState(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state[key]
	return v, ok
}

func (s *StateModule) DeleteState(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state, key)
}

func (s *StateModule) ClearState() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = make(map[string]interface{})
}

func (s *StateModule) GetAllState() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]interface{}, len(s.state))
	for k, v := range s.state {
		cp[k] = v
	}
	return cp
}
