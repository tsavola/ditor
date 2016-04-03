NAME		:= $(shell pwd | sed 's,^.*/src/,,')
ALL		:= $(NAME) $(NAME)/dup $(NAME)/edit

default:
	go fmt $(ALL)
	go vet $(ALL)

check: default
	go test $(ALL)

ifneq ($(GOPATH),)
build: default
	go get github.com/BurntSushi/freetype-go/freetype/truetype
	go get github.com/BurntSushi/xgb/xproto
	go get github.com/BurntSushi/xgbutil
	go get github.com/BurntSushi/xgbutil/keybind
	go get github.com/BurntSushi/xgbutil/xevent
	go get github.com/BurntSushi/xgbutil/xgraphics
	go build $(NAME)
endif

clean:
	rm -f ditor

.PHONY: default check build clean
