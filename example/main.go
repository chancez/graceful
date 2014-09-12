package main

import (
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ecnahc515/graceful"
)

func accept(l net.Listener, die chan struct{}) {
	for {
		select {
		case <-die:
			return
		default:
			conn, err := l.Accept()
			if err != nil {
				log.Fatal(err)
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
	die := make(chan struct{}, 1)
	signal.Notify(sigChan, syscall.SIGUSR2, syscall.SIGINT, syscall.SIGKILL)
	files := graceful.NewListenerFiles()
	defer func() {
		log.Println("Exiting")
		close(sigChan)
		close(die)
		files.CloseAll()
	}()

	for {
		l, err := graceful.NewGracefulListener("tcp", ":8080", files)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Accepting connections")
		go accept(l, die)

		// Wait for a signal
		sig := <-sigChan
		// all signals we close the connection, and stop the accept go routine.
		err = l.Close()
		if err != nil {
			log.Fatal("error closing", err)
		}
		die <- struct{}{}

		// if we're not just restarting, exit out the loop and cleanup
		if sig != syscall.SIGUSR2 {
			break
		}
	}
}
