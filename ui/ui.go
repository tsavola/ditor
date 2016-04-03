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
	defaultWidth  = 640
	defaultHeight = 480
	fontFilename  = "/usr/share/fonts/truetype/ubuntu-font-family/UbuntuMono-R.ttf"
	fontSize      = 20
	contIndent    = 8
	editBufSize   = 100 // BUG: full buffer causes deadlock
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

	font       *truetype.Font
	fontWidth  int
	fontHeight int
	xutil      *xgbutil.XUtil
	image      *xgraphics.Image
	winId      xproto.Window
	width      int
	height     int
}

func New(name string) (ui *UI, err error) {
	ui = &UI{
		Edits:  make(chan *edit.Edit, editBufSize),
		width:  defaultWidth,
		height: defaultHeight,
	}

	fontReader, err := os.Open(fontFilename)
	if err != nil {
		return
	}

	ui.font, err = xgraphics.ParseFont(fontReader)
	if err != nil {
		return
	}

	ui.fontWidth, ui.fontHeight = xgraphics.Extents(ui.font, fontSize, "g")

	ui.xutil, err = xgbutil.NewConn()
	if err != nil {
		return
	}

	keybind.Initialize(ui.xutil)

	ui.image = xgraphics.New(ui.xutil, image.Rect(0, 0, ui.width, ui.height))
	ui.image.For(func(x, y int) xgraphics.BGRA {
		return bgColor
	})

	win := ui.image.XShowExtra(name, true)
	win.Listen(xproto.EventMaskKeyPress)
	ui.winId = win.Id

	xevent.KeyPressFun(ui.handleKeyPress).Connect(ui.xutil, win.Id)

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
			ui.Edits <- &edit.Edit{Return: true}

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
	ui.image.For(func(x, y int) xgraphics.BGRA {
		return bgColor
	})

	widthColumns := ui.width / ui.fontWidth
	contColumns := widthColumns - contIndent
	c := editor.Caret
	y := 0

	for i, line := range editor.Lines() {
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
						Min: image.Point{x + ui.fontWidth*c.Column, y},
					}
					r.Max = image.Point{r.Min.X + ui.fontWidth, r.Min.Y + ui.fontHeight}
					fillRect(ui.image, r, caretColor)
				}
				c.Column -= columns
			}

			_, _, err = ui.image.Text(x, y, textColor, fontSize, ui.font, text)
			if err != nil {
				return
			}

			y += ui.fontHeight
			x = ui.fontWidth * contIndent
			columns = contColumns

			if len(line) == 0 {
				break
			}
		}
	}

	ui.image.XDraw()
	ui.image.XPaint(ui.winId)
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
