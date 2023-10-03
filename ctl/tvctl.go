/*
	tvctl - A daemon which receives key codes from Arduino and emulates keyboard actions according to a config file.

	Copyright (C) 2022~2023 Vadim Kuznetsov <vimusov@gmail.com>

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.
	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.
	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	portSpeed   uint32 = 9600
	repeatDelay        = 300 * time.Millisecond
)

type keyDesc struct {
	shortcut string
	comment  string
}

func openPort(name string) int {
	portFD, errOpen := unix.Open(name, unix.O_RDONLY|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if errOpen != nil {
		log.Fatalf("Unable open port: %v.", errOpen)
	}

	tios := unix.Termios{}
	tios.Cflag |= unix.CREAD | unix.CLOCAL | unix.BOTHER | unix.CS8
	tios.Ispeed = portSpeed
	tios.Ospeed = portSpeed
	tios.Iflag |= unix.INPCK
	tios.Cc[unix.VMIN] = 1
	tios.Cc[unix.VTIME] = 0

	if errTio := unix.IoctlSetTermios(portFD, unix.TCSETS2, &tios); errTio != nil {
		if errClose := unix.Close(portFD); errClose != nil {
			log.Fatalf("Unable close port: %v.", errClose)
		}
		log.Fatalf("Unable set flags: %v.", errTio)
	}
	return portFD
}

func loadConfig() (string, map[int]keyDesc) {
	homeDir, errHomeDir := os.UserHomeDir()
	if errHomeDir != nil {
		log.Fatalf("Unable to get home directory: %v.", errHomeDir)
	}

	cfgPath := filepath.Join(homeDir, ".config", "tvctl.conf")
	content, errRead := os.ReadFile(cfgPath)
	if errRead != nil {
		log.Fatalf("Unable to load config: %v.", errRead)
	}

	port := ""
	table := map[int]keyDesc{}
	for index, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)
		lineno := index + 1
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "/dev/") {
			if port != "" {
				log.Fatalf("Port is already defined as %q, error at line %d in %q.", port, lineno, cfgPath)
			}
			info, infoErr := os.Stat(line)
			if infoErr != nil {
				log.Fatalf("Wrong port value %q in %q, error %v.", line, cfgPath, infoErr)
			}
			if info.Mode()&fs.ModeCharDevice == 0 {
				log.Fatalf("%q is not a valid device in %q at line %d.", line, cfgPath, lineno)
			}
			port = line
			continue
		}
		keyPart, valPart, found := strings.Cut(line, ":")
		if !found {
			log.Fatalf("Invalid config, no separator ':' in %q at line %d.", cfgPath, lineno)
		}
		key, errConv := strconv.Atoi(strings.TrimSpace(keyPart))
		if errConv != nil {
			log.Fatalf("Wrong integer value %q in %q at line %d.", keyPart, cfgPath, lineno)
		}
		shortcut, comment, _ := strings.Cut(valPart, "#")
		table[key] = keyDesc{shortcut: strings.TrimSpace(shortcut), comment: strings.TrimSpace(comment)}
	}
	return port, table
}

func readCode(portFD int) int {
	buf := make([]byte, 8)
	size, readErr := unix.Read(portFD, buf)
	if readErr != nil {
		log.Fatalf("Unable to read: %v.", readErr)
	}
	data := string(buf[:size])
	value, errConv := strconv.Atoi(strings.TrimSpace(data))
	if errConv != nil {
		log.Fatalf("Invalid code value %q: %v.", data, errConv)
	}
	return value
}

func showCodes(portFD int, table map[int]keyDesc) {
	for {
		code := readCode(portFD)
		key, found := table[code]
		shortcut := key.shortcut
		if !found {
			fmt.Printf("%d: ?  # ?\n", code)
			continue
		}
		if key.comment == "" {
			fmt.Printf("%d: %s\n", code, shortcut)
		} else {
			fmt.Printf("%d: %s  # %s\n", code, shortcut, key.comment)
		}
	}
}

func processCommands(portFD int, table map[int]keyDesc) {
	lastTime := time.Now()
	for {
		code := readCode(portFD)
		curTime := time.Now()
		if curTime.Sub(lastTime) < repeatDelay {
			continue
		}
		lastTime = curTime
		key, found := table[code]
		if !found {
			continue
		}
		if errExec := exec.Command("xdotool", "key", key.shortcut).Run(); errExec != nil {
			log.Fatalf("Unable send shortcut %q: %v.", key.shortcut, errExec)
		}
	}
}

func notifySystemd() {
	path := os.Getenv("NOTIFY_SOCKET")
	if path == "" {
		return
	}
	addr := &net.UnixAddr{Name: path, Net: "unixgram"}
	conn, errDial := net.DialUnix(addr.Net, nil, addr)
	if errDial != nil {
		log.Fatalf("Unable open socket %q: %v.", path, errDial)
	}
	defer func() {
		if errClose := conn.Close(); errClose != nil {
			log.Fatalf("Unable close socket %q: %v.", path, errClose)
		}
	}()
	if _, errSend := conn.Write([]byte("READY=1")); errSend != nil {
		log.Fatalf("Unable send notify: %v.", errSend)
	}
}

func main() {
	debug := flag.Bool("debug", false, "Enable debug mode.")
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("FATAL: ")

	port, table := loadConfig()
	portFD := openPort(port)
	defer func() {
		if errClose := unix.Close(portFD); errClose != nil {
			log.Fatalf("Unable close port: %v.", errClose)
		}
	}()

	notifySystemd()

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	if *debug {
		go showCodes(portFD, table)
	} else {
		go processCommands(portFD, table)
	}
	<-signals
}
