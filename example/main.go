package main

import (
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ecnahc515/graceful"
)

var counter = 0

func accept(l net.Listener, die chan struct{}, done chan struct{}) {
	for {
		select {
		case <-die:
			log.Println("dying", counter)
			close(done)
			return
		default:
		}

		log.Println("accepting")
		if gl, ok := l.(*graceful.GracefulListener); ok {
			if tl, ok := gl.Listener.(*net.TCPListener); ok {
				log.Println("set deadline")
				tl.SetDeadline(time.Now().Add(1e9))
			}
		} else {
			log.Println("not graceful")
		}
		conn, err := l.Accept()
		log.Println("done accept")
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			log.Println(err)
		}

		// echo data back
		go func(c net.Conn) {
			io.Copy(c, c)
			c.Close()
		}(conn)
	}
}

func main() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGUSR2, syscall.SIGINT, syscall.SIGKILL)
	files := graceful.NewListenerFiles()
	defer func() {
		log.Println("Exiting")
		close(sigChan)
		files.CloseAll()
	}()

	for {
		log.Println("counter is", counter)
		l, err := graceful.NewGracefulListener("tcp", ":8080", files)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Accepting connections")
		die := make(chan struct{})
		done := make(chan struct{})
		go accept(l, die, done)

		// Wait for a signal
		sig := <-sigChan

		close(die)

		log.Println("kill")
		err = l.Close()
		if err != nil {
			log.Println("error closing listener", err)
		}
		<-done
		log.Println("died")

		counter++
		// if we're not just restarting, exit out the loop and cleanup
		if sig != syscall.SIGUSR2 {
			break
		}
	}
}
