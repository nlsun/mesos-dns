package admin

import (
	"os"
	"os/signal"
	"syscall"
	"github.com/mesosphere/mesos-dns/logging"
)

// Exposes a basic admin API
type Admin struct {
	Reload chan interface{}
}

func New() *Admin {
	admin := &Admin{
		Reload: make(chan interface{}, 1),
	}
	admin.setupSignalHandler()
	return admin
}

func (admin *Admin) setupSignalHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		defer signal.Stop(ch)
		defer close(ch)
		for range ch {
			select {
			case admin.Reload <- nil: logging.Verbose.Print("Reloading due to signal")
			default: logging.Verbose.Print("Tried to reload, but blocked")
			}
		}
	}()
}
