package main

import (
	// "bytes"
	// "fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var Reload os.Signal = syscall.SIGUSR1
var sigchan = make(chan os.Signal, 1)

func WritePidFile() {
	pid := os.Getpid()

	file, err := os.Create("smsrelay.pid")
	if err != nil {
		log.Fatal(err)
		os.Exit(-1)
	}

	file.WriteString(strconv.Itoa(pid))
	file.Close()
}

func ReopenLog() {
	file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		dlog.Println(err)
		return
	}

	// Set log output to file
	log.SetOutput(file)
	logFile.Close()
	logFile = file
}

func HandleReload() {
	dlog.Println("SIGUSR1 received")
	ReopenLog()
}

func SignalHandler() {
	for sig := range sigchan {
		switch sig {
		case Reload:
			HandleReload()
		}
	}
}

// A timer that updates Now every second
func UpdateTime() {
	for {
		Now = <-time.After(1 * time.Second)
		// fmt.Println(Now.Hour())
	}
}

func Encode(s1 []string, s2 []string) string {
	a := []string{}
	for i, s := range s1 {
		a = append(a, s+"="+s2[i])
	}
	return strings.Join(a, "&")
}
