package edit

type Editor struct {
	Text string
}

func (e *Editor) Init() {
	if e.Text == "" {
		e.Text = "<init>"
	}
}

func (e *Editor) Edit(text string) {
	e.Text = text
}
