package main

import (
	"github.com/ecnahc515/graceful"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func accept(l net.Listener, die chan struct{}) {
	for {
		select {
		case <-die:
			log.Println("dying")
			close(die)
			return
		default:
			if tl, ok := l.(*net.TCPListener); ok {
				tl.SetDeadline(time.Now().Add(time.Second))
			}
			conn, err := l.Accept()
			if err != nil {
				// Temporary error, just keep going
				if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
					continue
				}
				log.Fatal("error accepting", err)
			}

			// echo data back
			go func(c net.Conn) {
				io.Copy(c, c)
				c.Close()
			}(conn)
		}
	}
}

func main() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGUSR2, syscall.SIGINT, syscall.SIGKILL)

	defer func() {
		log.Println("Exiting")
		close(sigChan)
	}()

	manager := graceful.NewListenerManager()
	l, err := manager.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}

	// Used to toggle between accepting connections
	accepting := false

	var die chan struct{}

	for {
		log.Println("waiting for signal")
		sig := <-sigChan
		log.Println("got signal", sig)
		if sig == syscall.SIGUSR2 {
			if accepting {
				log.Println("killing")
				die <- struct{}{}
				accepting = false
			} else {
				log.Println("Accepting connections")
				die = make(chan struct{}, 1)
				go accept(l, die)
				accepting = true
			}
		} else if sig == syscall.SIGINT || sig == syscall.SIGKILL || sig == syscall.SIGTERM {
			l.Close()
			return
		}
	}
}
