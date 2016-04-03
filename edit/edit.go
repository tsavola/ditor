package edit

// Edit
type Edit struct {
	Control    bool
	MoveLine   int
	MoveColumn int
	Backspace  bool
	Return     bool
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
	Buffer [][]rune
	Caret  CaretPos
}

func (e *Editor) Init() {
	if len(e.Buffer) == 0 {
		e.Buffer = append(e.Buffer, nil)
		e.Caret.forget()
	}
	e.normalizeCaret()
}

func (e *Editor) Lines() (lines []string) {
	for _, buf := range e.Buffer {
		lines = append(lines, string(buf))
	}
	return
}

func (e *Editor) Apply(edit *Edit) {
	c := &e.Caret

	if edit.Control {
		switch edit.Char {
		case 'a':
			c.setColumn(0)

		case 'e':
			c.setColumn(len(e.Buffer[c.Line]))
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
		} else if c.Line > 0 {
			oldLine := e.Buffer[c.Line]
			e.Buffer = append(e.Buffer[:c.Line], e.Buffer[c.Line+1:]...)
			c.Line--
			c.setColumn(len(e.Buffer[c.Line]))
			e.Buffer[c.Line] = append(e.Buffer[c.Line], oldLine...)
		}
	} else if edit.Return {
		oldLine := e.Buffer[c.Line][:c.Column]
		newLine := e.Buffer[c.Line][c.Column:]
		e.Buffer[c.Line] = oldLine
		c.Line++
		c.setColumn(0)
		e.Buffer = append(e.Buffer[:c.Line], append([][]rune{newLine}, e.Buffer[c.Line:]...)...)
	} else {
		e.Buffer[c.Line] = append(e.Buffer[c.Line][:c.Column], append([]rune{edit.Char}, e.Buffer[c.Line][c.Column:]...)...)
		c.addColumn(1)
	}
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
