// Package dup uses Unix sockets and encoding/gob to copy program state from
// another program instance.
package dup

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// Duper duplicates an object hierarchy between processes.  Start with an empty
// state, optionally populate it with Import, and Export it for the duration of
// the program.
//
// The Name field and the Import method's parameter specify basenames of
// sockets.  The actual socket name will have a sequence number suffix.  When
// importing, the highest sequence number is preferred.  Export finds creates a
// socket with the next available sequence number.
type Duper struct {
	sync.Mutex             // Guards access to State.
	State      interface{} // Pointer to the root of the object hierarchy.
	SockDir    string      // A pre-existing directory.
	Name       string      // Import method sets this if not provided.

	listenName string
	listener   net.Listener
}

// Import requests State from another process.  If oldName is not empty, the
// latest socket didn't work, but an older one did.
func (d *Duper) Import(name string) (oldName string, err error) {
	name = d.fixName(name)

	nums, err := existingNums(name)
	if err != nil {
		return
	}

	names := []string{name}
	for _, num := range nums {
		names = append(names, fmt.Sprintf("%s.%d", name, num))
	}

	var c net.Conn
	var old bool

	for i := len(names) - 1; i >= 0; i-- {
		c, err = net.Dial("unix", names[i])
		if err == nil {
			if old {
				oldName = names[i]
			}
			break
		}
		old = true
	}
	if err != nil {
		return
	}
	defer c.Close()

	d.Lock()
	defer d.Unlock()

	err = gob.NewDecoder(c).Decode(d.State)
	if err != nil {
		return
	}

	if d.Name == "" {
		d.Name = name
	}

	return
}

// Export creates a server for serving our State to other processes.  Close
// closes it.
func (d *Duper) Export() (err error) {
	d.Name = d.fixName(d.Name)

	for {
		var nums []int
		var nextNum int

		nums, err = existingNums(d.Name)
		if err != nil {
			return
		}

		if len(nums) > 0 {
			nextNum = nums[len(nums)-1] + 1
		}

		nextName := fmt.Sprintf("%s.%d", d.Name, nextNum)

		d.listener, err = net.Listen("unix", nextName)
		if err == nil {
			d.listenName = nextName
			go d.acceptLoop(d.listener)
		} else if e, ok := err.(*net.OpError); ok {
			if e, ok := e.Err.(*os.SyscallError); ok && e.Err == syscall.EADDRINUSE {
				continue
			}
		}

		return
	}
}

func (d *Duper) acceptLoop(l net.Listener) {
	for {
		c, err := l.Accept()
		if d.listenName == "" {
			return
		}
		if err == nil {
			err = d.handleConn(c)
		}
		if err != nil {
			log.Print(err)
		}
	}
}

func (d *Duper) handleConn(c net.Conn) error {
	defer c.Close()

	d.Lock()
	defer d.Unlock()

	return gob.NewEncoder(c).Encode(d.State)
}

// Close the server and remove the socket file, if Export was called.
func (d *Duper) Close() (err error) {
	if d.listenName == "" {
		return
	}

	os.Remove(d.listenName)
	d.listenName = ""

	err = d.listener.Close()
	d.listener = nil
	return
}

func (d *Duper) fixName(name string) string {
	if strings.ContainsRune(name, os.PathSeparator) {
		return name
	} else {
		return path.Join(d.SockDir, name)
	}
}

func existingNums(name string) (nums []int, err error) {
	matches, err := filepath.Glob(name + ".*")
	if err != nil {
		return
	}

	for _, match := range matches {
		tokens := strings.Split(match, ".")
		if n, err := strconv.Atoi(tokens[len(tokens)-1]); err == nil {
			nums = append(nums, n)
		}
	}

	sort.Ints(nums)
	return
}
