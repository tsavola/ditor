NAME		:= $(shell pwd | sed 's,^.*/src/,,')
ALL		:= $(NAME) $(NAME)/dup $(NAME)/edit

default:
	go fmt $(ALL)
	go vet $(ALL)

check: default
	go test $(ALL)

ifneq ($(GOPATH),)
build: default
	go get golang.org/x/crypto/ssh/terminal
	go build $(NAME)
endif

clean:
	rm -f ditor

.PHONY: default check build clean
