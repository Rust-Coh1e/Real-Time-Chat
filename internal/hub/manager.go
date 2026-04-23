package hub

import "sync"

type HubManager struct {
	hubs map[string]*Hub
	mu   sync.RWMutex
}

func NewHubManager() *HubManager {
	return &HubManager{
		hubs: make(map[string]*Hub),
	}
}

func (h *HubManager) GetOrMake(name string) *Hub {
	h.mu.RLock()
	res, ok := h.hubs[name]
	if ok == true {
		h.mu.RUnlock()
		return res
	}
	h.mu.RUnlock()
	h.mu.Lock()
	defer h.mu.Unlock()
	res, ok = h.hubs[name]
	if ok == true {
		return res
	} else {
		h.hubs[name] = NewHub(name)
		go h.hubs[name].Run()
		res = h.hubs[name]
	}
	return res
}
