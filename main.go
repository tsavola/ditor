package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
	"syscall"

	"github.com/tsavola/ditor/dup"
	"github.com/tsavola/ditor/edit"
	"github.com/tsavola/ditor/ui"
)

const (
	appName     = "ditor"
	userRunDir  = "/run/user"
	defaultName = "default"
)

func init() {
	log.SetFlags(0)
}

func main() {
	os.Exit(mainResult())
}

func initSignals() (some chan os.Signal) {
	all := make(chan os.Signal)
	signal.Notify(all, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)

	some = make(chan os.Signal)

	go func() {
		defer close(some)

		for sig := range all {
			if sig == syscall.SIGQUIT {
				pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
			} else {
				some <- sig
			}
		}
	}()

	return
}

func mainResult() (status int) {
	signals := initSignals()

	editor := new(edit.Editor)

	duper := &dup.Duper{
		State:   editor,
		SockDir: fmt.Sprintf("%s/%d/%s", userRunDir, os.Getuid(), appName),
	}

	newState := flag.Bool("new", false, "start with an empty state (requires -n, ignores -s)")
	loadName := flag.String("s", defaultName, "load initial state from the named socket")
	fileName := flag.String("f", "", "open file (requires -new)")
	flag.StringVar(&duper.Name, "n", duper.Name, "name for the listening socket (defaults -s)")
	flag.StringVar(&duper.SockDir, "sockdir", duper.SockDir, "default location for sockets")

	flag.Parse()

	if *newState {
		if *fileName != "" {
			if err := editor.Open(*fileName); err != nil {
				log.Print(err)
				return 3
			}
		}
	} else {
		if *loadName == "" {
			flag.Usage()
			return 2
		}

		oldName, err := duper.Import(*loadName)
		if err != nil {
			log.Print(err)
			return 3
		}

		if oldName != "" {
			log.Printf("loading state from older instance: %s", oldName)
		}
	}

	editor.Init()

	if duper.Name == "" {
		flag.Usage()
		return 2
	}

	os.MkdirAll(duper.SockDir, 0700)

	err := duper.Export()
	if err != nil {
		log.Print(err)
		return 3
	}
	defer duper.Close()

	x, err := ui.New(editor.Name)
	if err != nil {
		log.Print(err)
		return 3
	}
	defer x.Close()

	if err := x.Refresh(editor); err != nil {
		log.Print(err)
		return 3
	}

	for {
		select {
		case <-x.BeforeEvent:
			<-x.AfterEvent

		case <-x.Stopped:
			return

		case edit, ok := <-x.Edits:
			if !ok {
				return
			}

			if err := handle(&duper.Mutex, editor, edit); err != nil {
				log.Print(err)
			}

			if err := x.Refresh(editor); err != nil {
				log.Print(err)
				return 3
			}

		case sig := <-signals:
			panic(sig)
		}
	}
}

func handle(m *sync.Mutex, editor *edit.Editor, edit *edit.Edit) error {
	m.Lock()
	defer m.Unlock()

	return editor.Apply(edit)
}
