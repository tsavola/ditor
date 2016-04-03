package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"ditor/dup"
	"ditor/edit"
)

const (
	appSubDir   = "ditor"
	userRunDir  = "/run/user"
	defaultName = "default"
)

func init() {
	log.SetFlags(0)
}

func main() {
	os.Exit(mainResult())
}

func mainResult() (status int) {
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGWINCH)

	editor := new(edit.Editor)

	duper := &dup.Duper{
		State:   editor,
		SockDir: fmt.Sprintf("%s/%d/%s", userRunDir, os.Getuid(), appSubDir),
	}

	newState := flag.Bool("new", false, "start with an empty state (requires -n, ignores -s)")
	loadName := flag.String("s", defaultName, "load initial state from the named socket")
	flag.StringVar(&duper.Name, "n", duper.Name, "name for the listening socket (defaults -s)")
	flag.StringVar(&duper.SockDir, "sockdir", duper.SockDir, "default location for sockets")

	flag.Parse()

	if !*newState {
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
	log.Printf("initial: %#v", editor)

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

	if !terminal.IsTerminal(syscall.Stdin) {
		log.Print("stdin is not a terminal")
		return 3
	}
	origTermState, err := terminal.MakeRaw(syscall.Stdin)
	if err != nil {
		log.Print(err)
		return 3
	}
	defer terminal.Restore(syscall.Stdin, origTermState)
	term := terminal.NewTerminal(os.Stdin, "content: ")

	errors := make(chan error, 2)
	defer func() {
		select {
		case err := <-errors:
			log.Print("")
			log.Print(err)
			status = 3

		default:
			log.Print("")
		}
	}()

	termInput := make(chan string)
	go func() {
		defer close(termInput)
		for {
			line, err := term.ReadLine()
			if err != nil {
				if err != io.EOF {
					errors <- err
				}
				return
			}
			termInput <- line
		}
	}()

	for {
		select {
		case sig := <-signals:
			switch sig {
			case syscall.SIGWINCH:
				width, height, err := terminal.GetSize(syscall.Stdin)
				if err != nil {
					errors <- err
					return
				}

				if err := term.SetSize(width, height); err != nil {
					errors <- err
					return
				}

			default:
				panic(sig)
			}

		case line, ok := <-termInput:
			if !ok {
				return
			}
			handleInput(&duper.Mutex, editor, line)
		}
	}
}

func handleInput(m *sync.Mutex, e *edit.Editor, s string) {
	m.Lock()
	defer m.Unlock()

	e.Edit(s)
}
