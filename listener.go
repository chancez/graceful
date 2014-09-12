package graceful

import (
	"errors"
	"net"
	"os"
	"sync"
	"syscall"
)

var ErrAlreadyClosed = errors.New("already closed")

type FileListener interface {
	File() (*os.File, error)
}

// We wrap a file so we can keep track of if something is using a file.
type ListenerFile struct {
	file     *os.File
	refCount int
}

type ListenerFiles struct {
	files map[ListenerMetaData]ListenerFile
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

func (f ListenerFile) File() *os.File {
	return f.file
}

func (f ListenerFile) RefCount() int {
	return f.refCount
}

func NewListenerFiles() *ListenerFiles {
	return &ListenerFiles{files: make(map[ListenerMetaData]ListenerFile)}
}

func (lf *ListenerFiles) GetFile(meta ListenerMetaData) *os.File {
	lf.mutex.RLock()
	defer lf.mutex.RUnlock()
	if f, ok := lf.files[meta]; ok {
		f.refCount++
		return f.file
	}
	return nil
}

func (lf *ListenerFiles) SetFile(meta ListenerMetaData, file *os.File) {
	lf.mutex.Lock()
	lf.files[meta] = ListenerFile{file: file}
	lf.mutex.Unlock()
}

func (lf *ListenerFiles) CloseAll() {
	lf.mutex.Lock()
	for _, f := range lf.files {
		if f.file != nil {
			f.file.Close()
		}
	}
	lf.mutex.Unlock()
}

// New GracefulListener creates and returns a wrapped net.Listener. It reuses
// file descriptors in `files` if the meta data matches the lnet and laddr.
func NewGracefulListener(lnet, laddr string, files *ListenerFiles) (net.Listener, error) {
	if files == nil {
		return nil, errors.New("ListenerFiles cannot be nil")
	}
	listenerMeta := ListenerMetaData{net: lnet, laddr: laddr}

	var (
		listener net.Listener
		err      error
	)

	f := files.GetFile(listenerMeta)
	// Check if this listener already exists
	if f != nil {
		// Reuse an existing listener's socket
		listener, err = net.FileListener(f)
		if err != nil {
			return nil, err
		}
	} else {
		listener, err = net.Listen(lnet, laddr)
		if err != nil {
			return nil, err
		}
		if fl, ok := listener.(FileListener); ok {
			f, err = fl.File()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("No file associated with this listener")
		}
	}
	files.SetFile(listenerMeta, f)
	return &GracefulListener{Listener: listener}, nil
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
	defer c.once.Do(c.wg.Done)
	return c.Conn.Close()
}
