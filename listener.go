package graceful

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
)

var (
	// globalListenerFiles is used to track all listeners, and their open socket files
	// If it exists in this map, then the file should be re-used, unless the
	// MetaData has changed. If it's changed, when all plugins are done loading,
	// the files without listeners should be pruned.
	globalListenerFiles ListenerFiles

	ErrAlreadyClosed = errors.New("already closed")
)

func init() {
	globalListenerFiles.files = make(map[ListenerMetaData]*os.File)
}

type FileListener interface {
	File() (*os.File, error)
}

type ListenerFiles struct {
	files map[ListenerMetaData]*os.File
	mutex sync.RWMutex
}

type ListenerMetaData struct {
	net, laddr string
}

type GracefulListener struct {
	Listener    net.Listener
	closed      bool
	closedMutex sync.RWMutex
	wg          sync.WaitGroup
}

type gracefulConn struct {
	net.Conn
	wg   *sync.WaitGroup
	once sync.Once
}

func (lf *ListenerFiles) GetFile(meta ListenerMetaData) *os.File {
	lf.mutex.RLock()
	defer lf.mutex.RUnlock()
	if f, ok := lf.files[meta]; ok {
		return f
	}
	return nil
}

func (lf *ListenerFiles) SetFile(meta ListenerMetaData, file *os.File) {
	lf.mutex.Lock()
	lf.files[meta] = file
	lf.mutex.Unlock()
}

func CloseAll() {
	lf := globalListenerFiles
	lf.mutex.Lock()
	for _, file := range lf.files {
		if file != nil {
			file.Close()
		}
	}
	lf.mutex.Unlock()
}

// New GracefulListener creates a new net.Listener and returns a
// GracefulListener which wraps the original listener. The listeners lnet,
// and laddr are used as metadata to track existing sockets to be reused by
// NewGracefulListener.
func NewGracefulListener(lnet, laddr string) (net.Listener, error) {
	listenerMeta := ListenerMetaData{net: lnet, laddr: laddr}
	var (
		listener net.Listener
		err      error
	)
	f := globalListenerFiles.GetFile(listenerMeta)
	// Check if this listener already exists
	if f != nil {
		// Reuse an existing listener's socket
		fmt.Println("reusing file")
		listener, err = net.FileListener(f)
		if err != nil {
			return nil, err
		}
	} else {
		fmt.Println("new listener")
		listener, err = net.Listen(lnet, laddr)
		if err != nil {
			return nil, err
		}
		if fl, ok := listener.(FileListener); ok {
			f, err = fl.File()
			if err != nil {
				return nil, err
			}
		}
	}
	// wrap the listener
	l := &GracefulListener{Listener: listener}
	globalListenerFiles.SetFile(listenerMeta, f)
	return l, nil
}

func (l *GracefulListener) Accept() (c net.Conn, err error) {
	l.closedMutex.RLock()
	if l.closed {
		l.closedMutex.RLock()
		return nil, ErrAlreadyClosed
	}
	l.closedMutex.RUnlock()

	c, err = l.Listener.Accept()
	if err != nil {
		return
	}

	c = &gracefulConn{Conn: c, wg: &l.wg}
	l.wg.Add(1)
	return
}

func (l *GracefulListener) Close() (err error) {
	if l.closed {
		return syscall.EINVAL
	}

	l.closed = true
	err = l.Listener.Close()
	// l.wg.Wait()
	return
}

func (l *GracefulListener) Addr() net.Addr {
	return l.Listener.Addr()
}

func (c *gracefulConn) Close() error {
	fmt.Println("con close")
	defer c.once.Do(c.wg.Done)
	return c.Conn.Close()
}
