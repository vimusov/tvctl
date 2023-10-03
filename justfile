target := "tvctl"

default: build

flash:
    cd fw && pio run --target upload

build:
    cd ctl && go build

clean:
    rm -f ctl/{{target}}

install destdir: build
    install -D -m 0755 ctl/{{target}} "{{destdir}}"/usr/bin/{{target}}
    install -D -m 0644 contrib/config "{{destdir}}"/usr/share/tvctl/example.conf
    install -D -m 0644 contrib/service "{{destdir}}"/usr/lib/systemd/user/{{target}}.service
