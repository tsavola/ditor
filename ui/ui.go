package ui

import (
	"image"
	"log"
	"os"
	"strings"

	"github.com/BurntSushi/freetype-go/freetype/truetype"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xgraphics"

	"ditor/edit"
)

const (
	fontFilename   = "/usr/share/fonts/truetype/ubuntu-font-family/UbuntuMono-R.ttf"
	fontSize       = 20
	defaultColumns = 80
	defaultLines   = 30
	contIndent     = 6
	editBufSize    = 100 // BUG: full buffer causes deadlock
)

var (
	bgColor    = xgraphics.BGRA{0x00, 0x00, 0x00, 0xff}
	textColor  = xgraphics.BGRA{0xff, 0xff, 0xff, 0xff}
	caretColor = xgraphics.BGRA{0xff, 0x7f, 0x00, 0xff}
)

// UI
type UI struct {
	BeforeEvent <-chan struct{}
	AfterEvent  <-chan struct{}
	Stopped     <-chan struct{}
	Edits       chan *edit.Edit

	font     *truetype.Font
	fontSize image.Point
	winSize  image.Point
	xutil    *xgbutil.XUtil
	winImage *xgraphics.Image
	winId    xproto.Window
	topLine  int
}

func New(name string) (ui *UI, err error) {
	ui = &UI{
		Edits: make(chan *edit.Edit, editBufSize),
	}

	fontReader, err := os.Open(fontFilename)
	if err != nil {
		return
	}

	ui.font, err = xgraphics.ParseFont(fontReader)
	if err != nil {
		return
	}

	ui.fontSize.X, ui.fontSize.Y = xgraphics.Extents(ui.font, fontSize, "g")
	ui.winSize = image.Point{ui.fontSize.X*defaultColumns, ui.fontSize.Y*defaultLines}

	ui.xutil, err = xgbutil.NewConn()
	if err != nil {
		return
	}

	keybind.Initialize(ui.xutil)

	ui.winImage = xgraphics.New(ui.xutil, image.Rectangle{Max: ui.winSize})
	ui.winImage.For(func(x, y int) xgraphics.BGRA {
		return bgColor
	})

	win := ui.winImage.XShowExtra(name, true)
	win.Listen(xproto.EventMaskKeyPress)
	ui.winId = win.Id

	xevent.KeyPressFun(ui.handleKeyPress).Connect(ui.xutil, ui.winId)

	ui.BeforeEvent, ui.AfterEvent, ui.Stopped = xevent.MainPing(ui.xutil)
	return
}

func (ui *UI) handleKeyPress(xutil *xgbutil.XUtil, e xevent.KeyPressEvent) {
	var control bool

	mod := keybind.ModifierString(e.State)

	switch mod {
	case "control":
		control = true
		fallthrough

	case "", "shift":
		key := keybind.LookupString(xutil, e.State, e.Detail)

		switch key {
		case "Up":
			ui.Edits <- &edit.Edit{MoveLine: -1}

		case "Down":
			ui.Edits <- &edit.Edit{MoveLine: 1}

		case "Left":
			ui.Edits <- &edit.Edit{MoveColumn: -1}

		case "Right":
			ui.Edits <- &edit.Edit{MoveColumn: 1}

		case "BackSpace":
			ui.Edits <- &edit.Edit{Backspace: true}

		case "Return":
			ui.Edits <- &edit.Edit{Char: '\n'}

		case "Tab":
			ui.Edits <- &edit.Edit{Char: '\t'}

		default:
			if len(key) == 1 {
				if ch, _, err := strings.NewReader(key).ReadRune(); err == nil {
					ui.Edits <- &edit.Edit{
						Control: control,
						Char:    ch,
					}
				} else {
					log.Print(err)
				}
			} else {
				log.Print(key)
			}
		}

	default:
		log.Print(mod)
	}
}

func (ui *UI) Refresh(editor *edit.Editor) (err error) {
	widthColumns := ui.winSize.X / ui.fontSize.X
	contColumns := widthColumns - contIndent

	if editor.Caret.Line < ui.topLine {
		ui.topLine = editor.Caret.Line
	}

	for {
		restart := false

		ui.winImage.For(func(x, y int) xgraphics.BGRA {
			return bgColor
		})

		c := editor.Caret
		y := 0

		for i, line := range editor.Lines()[ui.topLine:] {
			i += ui.topLine

			if y >= ui.winSize.Y && c.Line < i {
				if offset := (y - ui.winSize.Y) / ui.fontSize.Y; offset > 0 {
					if top := ui.topLine + offset; top < editor.Caret.Line {
						ui.topLine = top
						restart = true
					}
				}
				break
			}

			indenting := true

			for n := 0; n < len(line); {
				if line[n] == '\t' {
					var width int
					if indenting {
						width = editor.Indent
					} else {
						width = edit.TabWidth - (n % edit.TabWidth)
					}

					prefix := line[:n]
					suffix := line[n+1:]

					var tab []byte
					for m := 0; m < width; m++ {
						tab = append(tab, ' ')
					}

					line = string(append([]byte(prefix), append(tab, []byte(suffix)...)...))

					if i == c.Line && c.Column > n {
						c.Column += width - 1
					}

					n += width
				} else {
					indenting = false
					n++
				}
			}

			x := 0
			columns := widthColumns

			for {
				text := line
				if len(text) > columns {
					text = line[:columns]
					line = line[columns:]
				} else {
					line = ""
				}

				if i == c.Line {
					if c.Column >= 0 && c.Column < columns {
						r := image.Rectangle{
							Min: image.Point{x + ui.fontSize.X*c.Column, y},
						}
						r.Max = image.Point{r.Min.X + ui.fontSize.X, r.Min.Y + ui.fontSize.Y}
						fillRect(ui.winImage, r, caretColor)
					}
					c.Column -= columns
				}

				_, _, err = ui.winImage.Text(x, y, textColor, fontSize, ui.font, text)
				if err != nil {
					return
				}

				y += ui.fontSize.Y
				x = ui.fontSize.X * contIndent
				columns = contColumns

				if len(line) == 0 {
					break
				}
			}
		}

		if !restart {
			break
		}
	}

	log.Printf("caret=%d top=%d", editor.Caret.Line, ui.topLine)

	ui.winImage.XDraw()
	ui.winImage.XPaint(ui.winId)
	return
}

func (ui *UI) Close() {
	xevent.Quit(ui.xutil)
}

func fillRect(i *xgraphics.Image, r image.Rectangle, c xgraphics.BGRA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i.Set(x, y, c)
		}
	}
}
