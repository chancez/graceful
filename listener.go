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

func (m *ListenerManager) Get(addr string) (l net.Listener) {
	m.RLock()
	l = m.m[addr]
	m.RUnlock()
	return
}

func (m *ListenerManager) Set(addr string, l net.Listener) {
	m.Lock()
	m.m[addr] = l
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
	return NewListener(lnet, laddr, m)
}

func NewListener(lnet, laddr string, m *ListenerManager) (l net.Listener, err error) {
	l, err = net.Listen(lnet, laddr)
	if err != nil {
		return
	}
	addr := l.Addr().String()
	m.Set(addr, l)
	return
}

func Listen(lnet, laddr string) (net.Listener, error) {
	return NewListener(lnet, laddr, NewListenerManager())
}
