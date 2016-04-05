package edit

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
)

const (
	TabWidth = 8

	defaultIndent = 4
	defaultName   = "*scratch*"
	tempPrefix    = ".ditor"
)

// Edit
type Edit struct {
	Control    bool
	MoveLine   int
	MoveColumn int
	Backspace  bool
	Char       rune
}

// Pos
type Pos struct {
	Line   int
	Column int
}

// CaretPos
type CaretPos struct {
	Pos
	RememberColumn int
}

func (c *CaretPos) reset() {
	c.Pos = Pos{}
	c.forget()
}

func (c *CaretPos) forget() {
	c.RememberColumn = -1
}

func (c *CaretPos) setColumn(n int) {
	c.Column = n
	c.forget()
}

func (c *CaretPos) addColumn(n int) {
	c.Column += n
	c.forget()
}

// Editor
type Editor struct {
	Buffer   [][]rune
	Caret    CaretPos
	Indent   int
	Name     string
	Filename string
	FilePerm os.FileMode
	Dirty    bool
}

func (e *Editor) Open(filename string) (err error) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return
	}

	r := bufio.NewReader(f)
	buf := [][]rune{nil}

	for {
		ch, _, e := r.ReadRune()
		if e != nil {
			if e == io.EOF {
				break
			}
			err = e
			return
		}

		if ch == '\n' {
			buf = append(buf, nil)
		} else {
			i := len(buf) - 1
			buf[i] = append(buf[i], ch)
		}
	}

	e.Buffer = buf
	e.Caret.reset()
	e.Indent = defaultIndent
	e.Name = path.Base(filename)
	e.Filename = filename
	e.FilePerm = info.Mode() & os.ModePerm
	e.Dirty = false
	return
}

func (e *Editor) Init() {
	if len(e.Buffer) == 0 {
		e.Buffer = append(e.Buffer, nil)
		e.Caret.forget()
		e.Indent = defaultIndent
		e.Name = defaultName
		e.Filename = ""
		e.FilePerm = 0
		e.Dirty = false
	}
	e.normalizeCaret()
}

func (e *Editor) Save() (err error) {
	if !e.Dirty {
		return
	}

	if e.Filename == "" {
		err = errors.New("no filename")
		return
	}

	dir := path.Dir(e.Filename)
	if dir == "" {
		dir = "."
	}

	f, err := ioutil.TempFile(dir, tempPrefix)
	if err != nil {
		return
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(f.Name())
		}
	}()

	w := bufio.NewWriter(f)

	for i, line := range e.Buffer {
		for _, ch := range line {
			_, err = w.WriteRune(ch)
			if err != nil {
				return
			}
		}

		if i < len(e.Buffer)-1 {
			_, err = w.WriteRune('\n')
			if err != nil {
				return
			}
		}
	}

	err = w.Flush()
	if err != nil {
		return
	}

	err = f.Chmod(e.FilePerm)
	if err != nil {
		return
	}

	err = os.Rename(f.Name(), e.Filename)
	if err != nil {
		return
	}

	e.Dirty = false
	return
}

func (e *Editor) Lines() (lines []string) {
	for _, buf := range e.Buffer {
		lines = append(lines, string(buf))
	}
	return
}

func (e *Editor) Apply(edit *Edit) (err error) {
	c := &e.Caret

	if edit.Control {
		switch edit.Char {
		case 'a':
			c.setColumn(0)

		case 'e':
			c.setColumn(len(e.Buffer[c.Line]))

		case 's':
			err = e.Save()
		}
	} else if edit.MoveLine != 0 {
		if i := c.Line + edit.MoveLine; i >= 0 && i < len(e.Buffer) {
			c.Line = i
			if c.RememberColumn < 0 {
				c.RememberColumn = c.Column
			} else {
				c.Column = c.RememberColumn
			}
			if n := len(e.Buffer[c.Line]); c.Column > n {
				c.Column = n // preserve RememberColumn
			}
		}
	} else if edit.MoveColumn != 0 {
		c.addColumn(edit.MoveColumn)
		e.normalizeCaret()
	} else if edit.Backspace {
		if c.Column > 0 {
			e.Buffer[c.Line] = append(e.Buffer[c.Line][:c.Column-1], e.Buffer[c.Line][c.Column:]...)
			c.addColumn(-1)
			e.Dirty = true
		} else if c.Line > 0 {
			oldLine := e.Buffer[c.Line]
			e.Buffer = append(e.Buffer[:c.Line], e.Buffer[c.Line+1:]...)
			c.Line--
			c.setColumn(len(e.Buffer[c.Line]))
			e.Buffer[c.Line] = append(e.Buffer[c.Line], oldLine...)
			e.Dirty = true
		}
	} else {
		switch edit.Char {
		case '\n':
			oldLine := e.Buffer[c.Line][:c.Column]
			newLine := append([]rune(nil), e.Buffer[c.Line][c.Column:]...)
			e.Buffer[c.Line] = oldLine
			c.Line++
			c.setColumn(0)
			e.Buffer = append(e.Buffer[:c.Line], append([][]rune{newLine}, e.Buffer[c.Line:]...)...)

		case '\t':
			column := 0
			indent := true
			for n := 0; n < c.Column; n++ {
				if e.Buffer[c.Line][n] == '\t' {
					column += TabWidth
				} else {
					column++
					indent = false
				}
			}
			if indent {
				e.insertChar('\t')
			} else {
				for n := 0; n < TabWidth-(column%TabWidth); n++ {
					e.insertChar(' ')
				}
			}

		default:
			e.insertChar(edit.Char)
		}
		e.Dirty = true
	}

	return
}

func (e *Editor) insertChar(ch rune) {
	c := &e.Caret

	e.Buffer[c.Line] = append(e.Buffer[c.Line][:c.Column], append([]rune{ch}, e.Buffer[c.Line][c.Column:]...)...)
	c.addColumn(1)
}

func (e *Editor) normalizeCaret() {
	c := &e.Caret

	if c.Line < 0 {
		c.Line = 0
		c.setColumn(0)
	} else if i := len(e.Buffer); c.Line >= i {
		c.Line = i - 1
		c.Column = len(e.Buffer[c.Line]) // preserve RememberColumn
	}

	for c.Column < 0 {
		if c.Line == 0 {
			c.setColumn(0)
			break
		}
		c.Line--
		c.setColumn(len(e.Buffer[c.Line]) + c.Column + 1)
	}

	for {
		n := len(e.Buffer[c.Line])
		if c.Column <= n {
			break
		}
		if c.Line == len(e.Buffer)-1 {
			c.setColumn(n)
			break
		}
		c.setColumn(n - c.Column + 1)
		c.Line++
	}
}
