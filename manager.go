package graceful

import (
	"net"
	"sync"
)

type ListenerManager struct {
	m map[string]net.Listener
	sync.RWMutex
}

func NewListenerManager() *ListenerManager {
	return &ListenerManager{m: make(map[string]net.Listener)}
}

func (m *ListenerManager) Get(lnet, laddr string) (l net.Listener) {
	m.RLock()
	l = m.m[lnet+laddr]
	m.RUnlock()
	return
}

func (m *ListenerManager) Set(lnet, laddr string, l net.Listener) {
	m.Lock()
	m.m[lnet+laddr] = l
	m.Unlock()
	return
}

func (m *ListenerManager) Listeners() []net.Listener {
	listeners := make([]net.Listener, 0, len(m.m))
	for _, l := range m.m {
		listeners = append(listeners, l)
	}
	return listeners
}

func (m *ListenerManager) Listen(lnet, laddr string) (net.Listener, error) {
	l := m.Get(lnet, laddr)
	if l != nil {
		return l, nil
	}
	return NewListener(lnet, laddr, m)
}

func NewListener(lnet, laddr string, m *ListenerManager) (l net.Listener, err error) {
	l, err = net.Listen(lnet, laddr)
	if err != nil {
		return
	}
	m.Set(lnet, laddr, l)
	return
}

func Listen(lnet, laddr string) (net.Listener, error) {
	return NewListener(lnet, laddr, NewListenerManager())
}
