BINARY   = scout
INSTALL  = /usr/local/bin/$(BINARY)
MANDIR   = /usr/local/share/man/man1
MANPAGE  = docs/scout.1
VERSION  ?= dev
LDFLAGS  = -s -w -X github.com/pavlitoss/scout-cli/cmd.version=$(VERSION)

.PHONY: build install uninstall clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./main.go

install: build
	sudo cp $(BINARY) $(INSTALL)
	sudo mkdir -p $(MANDIR)
	sudo cp $(MANPAGE) $(MANDIR)/scout.1
	sudo mandb -q
	@echo "Installed $(INSTALL) and man page"

uninstall:
	sudo rm -f $(INSTALL)
	sudo rm -f $(MANDIR)/scout.1
	sudo mandb -q
	@echo "Removed $(INSTALL) and man page"

clean:
	rm -f $(BINARY)
