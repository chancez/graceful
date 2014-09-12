package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ecnahc515/graceful"
)

func main() {
	sigChan := make(chan os.Signal)
	die := make(chan struct{}, 1)
	signal.Notify(sigChan, syscall.SIGUSR2, syscall.SIGINT, syscall.SIGKILL)
	defer func() {
		close(sigChan)
		close(die)
		graceful.CloseAll()
	}()

	for {
		fmt.Println("new graceful")
		l, err := graceful.NewGracefulListener("tcp", ":8080")
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("go listener")
		go func() {
			log.Println("Listening")
			for {
				select {
				case <-die:
					fmt.Println("told to this listener to die")
					return
				default:
					conn, err := l.Accept()
					if err != nil {
						log.Fatal(err)
					}

					log.Println("accepted conn", conn)
					go func(c net.Conn) {
						io.Copy(c, c)
						c.Close()
					}(conn)
				}
			}
		}()

		fmt.Println("before sig for")
		for {
			sig := <-sigChan
			fmt.Println("got a signal", sig)
			if sig == syscall.SIGUSR2 {
				fmt.Println("closing")
				err := l.Close()
				if err != nil {
					fmt.Println("error closing", err)
				}
				die <- struct{}{}
				break
			} else {
				log.Println("exiting")
				os.Exit(0)
			}
		}
		fmt.Println("after sig for")
	}
}
