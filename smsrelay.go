package main

import (
	"encoding/json"
	"flag"
	"fmt"
	// "io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	// "regexp"
	"errors"
	"strings"
	"time"
)

// Constants
const BUFFER_SIZE = 10000         // buffer size per channel
const MAX_RETRY = 3               // max retry for each sms
const WAIT_TIME = 3 * time.Second // wait time between each retry

// Flags
var verbose = flag.Bool("v", false, "verbose mode enables debug log")
var dlogPath = flag.String("d", "smsrelay.debug.log", "path to debug log")
var listen = flag.String("l", ":8180", "listen on [address]:port")
var configPath = flag.String("c", "smsrelay.conf", "path to config file")
var logPath = flag.String("o", "smsrelay.log", "path to log file")

// Debug log facility
var dlog *log.Logger

// Log file
var logFile *os.File

// Current time, updated every second
var Now = time.Now()

// SmsRelay interface
type SmsRelay interface {
	send(*Sms) (*http.Response, error)
	receive() (*http.Response, error)
	processSendResult([]byte) bool
	processReceiveResult([]byte) bool
	checkBalance() string
}

// Incoming messages need to implement UserMessage interface
type UserMessage interface {
	// Return non URL-encoded body for post
	genBody() string
}

// SMS data
type Sms struct {
	mobile    string
	message   string
	relayName string
	count     int
	from      string
	task      string
}

// SMS API Gateway config data
type GatewayConfig struct {
	URL           string
	BalanceURL    string
	ReceiveURL    string
	StatusURL     string
	CallerNumbers []string
}

// Relay config data
type RelayConfig struct {
	Gateway        string
	Userid         string
	Password       string
	Throughput     int
	GetSMSInterval int
}

// User config data
type UserConfig struct {
	Password  string
	Relay     string
	Extension string
	StartHour int
	StopHour  int
}

// Settings data
type SettingsConfig struct {
	DefaultRelay string
	SharedSecret string
	IncomingURL  string
}

// Config data
type Config struct {
	Gateways map[string]GatewayConfig
	Relays   map[string]RelayConfig
	Users    map[string]UserConfig
	Settings SettingsConfig
}

var config Config

// Load config file (JSON format)
func (conf *Config) LoadConfig(path string) (err error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("Failed to open config file:", path)
		return
	}
	err = json.Unmarshal(f, &conf)
	if err != nil {
		fmt.Println("Bad json:", err)
	}

	// Check default relay
	if _, ok := config.Relays[config.Settings.DefaultRelay]; !ok {
		err = errors.New("incorrect default relay in config file")
		fmt.Println("Misconfiguration:", err)
		return
	}

	// Check relay for users
	for name, user := range config.Users {
		if user.Relay != "default" {
			if _, ok := config.Relays[user.Relay]; !ok {
				err = errors.New("incorrect relay for user")
				fmt.Println("Misconfiguration:", err, name)
				return
			}
		}
	}
	return
}

// Sms queues for each relay
var outgoingQueues map[string]chan *Sms
var incomingQueue chan UserMessage

// Send Sms
func send(s *Sms) bool {
	relay := GetRelay(s.relayName)
	if relay != nil {
		resp, err := relay.send(s)
		if err != nil {
			dlog.Println(err)
			return false
		} else {
			defer resp.Body.Close()

			body, _ := ioutil.ReadAll(resp.Body)
			dlog.Printf("%s\n", body)

			return relay.processSendResult(body)
		}
	}

	return false
}

// Receive sms from user
func receive(relayName string) bool {
	relay := GetRelay(relayName)
	if relay != nil {
		resp, err := relay.receive()
		if err != nil {
			dlog.Println(err)
			return false
		} else {
			defer resp.Body.Close()

			body, _ := ioutil.ReadAll(resp.Body)
			// dlog.Printf("%s\n", body)

			return relay.processReceiveResult(body)
		}
	}

	return false
}

// Start outgoing jobs for each relay
func startOutgoingJobs() {
	for relayName, queue := range outgoingQueues {
		go outgoingSchedular(queue, config.Relays[relayName].Throughput)
	}
}

// Schedule outgoing jobs based on throughput limit of relay
func outgoingSchedular(c chan *Sms, throughput int) {
	for {
		s := <-c
		// hasVariable, _ := regexp.MatchString("[{}$]+", s.message)
		if Now.Hour() >= config.Users[s.from].StartHour && Now.Hour() < config.Users[s.from].StopHour {
			go sendWithRetry(s)
			time.Sleep(time.Duration(1000/throughput) * time.Millisecond)
		} else {
			log.Printf("dropped|0|%s|%s|%s|%s|%s|%d\n", s.from, s.task, s.relayName, s.mobile, s.message, s.count)
		}
	}
}

// Send SMS with retry
func sendWithRetry(s *Sms) {
	ret := send(s)
	retry := 0
	for ; !ret && retry < MAX_RETRY; retry++ {
		time.Sleep(WAIT_TIME)
		ret = send(s)
	}
	log.Printf("%t|%d|%s|%s|%s|%s|%s|%d\n", ret, retry, s.from, s.task, s.relayName, s.mobile, s.message, s.count)
}

// Add SMS to outgoing queue
func addToOutgoingQueue(s *Sms) {
	c := outgoingQueues[s.relayName]
	c <- s
}

// Start incoming jobs for each relay
func startIncomingJobs() {
	for relayName, _ := range config.Relays {
		go incomingSchedular(relayName, config.Relays[relayName].GetSMSInterval)
	}
}

// Schedule incoming jobs
func incomingSchedular(relayName string, interval int) {
	go dispatchIncomingMessages()
	for {
		go receive(relayName)
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

// Dispatch incoming messages to IncomingURL
func dispatchIncomingMessages() {
	for msg := range incomingQueue {
		dlog.Println("Sending", msg.genBody())
		resp, err := http.Post(config.Settings.IncomingURL,
			"application/x-www-form-urlencoded",
			strings.NewReader(msg.genBody()))
		if err != nil {
			dlog.Println("Error when posting incoming msg:", err)
		} else {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			dlog.Printf("%d: %s\n", resp.StatusCode, body)
		}
	}
}

// send API Handler
func sendHandler(w http.ResponseWriter, r *http.Request) {
	var params url.Values

	// Parse params
	if r.Method == "GET" {
		params = r.URL.Query()
	} else {
		body, err := ioutil.ReadAll(r.Body)

		if err != nil {
			dlog.Println(err)
		}

		params, err = url.ParseQuery(string(body))

		if err != nil {
			dlog.Println(err)
		}
	}

	// Check user
	from := params.Get("user")
	user, ok := config.Users[from]
	password := params.Get("password")

	// If the password is correct
	if ok && user.Password == password {
		var relayName string
		if user.Relay == "default" {
			relayName = config.Settings.DefaultRelay
		} else {
			relayName = user.Relay
		}

		// Construct sms data
		s := Sms{
			params.Get("mobile"),
			params.Get("message"),
			relayName,
			len(strings.Split(params.Get("mobile"), ",")),
			from,
			params.Get("task"),
		}

		// Add to queue for relay
		addToOutgoingQueue(&s)

		fmt.Fprintf(w, "%s", s)
	} else {
		fmt.Fprintf(w, "Sms refused: auth failed or incorrect params.")
	}
}

// balance API Handler
func balanceHandler(w http.ResponseWriter, r *http.Request) {
	balances := make(map[string]string)
	for name, _ := range config.Relays {
		relay := GetRelay(name)
		balances[name] = relay.checkBalance()
	}
	body, err := json.Marshal(balances)
	if err != nil {
		fmt.Fprintf(w, "Failed to marshal json response")
	} else {
		fmt.Fprintf(w, "%s", body)
	}
}

func main() {
	// Parse command-line flags
	flag.Parse()

	// Log debug info if -v flag is used
	if *verbose {
		dlogFile, err := os.OpenFile(*dlogPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal(err)
		}
		dlog = log.New(dlogFile, "", log.LstdFlags|log.Lshortfile)
	} else {
		dlog = log.New(ioutil.Discard, "", log.LstdFlags)
	}

	// Enable logging
	file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	logFile = file
	defer logFile.Close()

	// Set log output to file
	log.SetOutput(file)

	// Write pid file
	WritePidFile()

	// Initialize config
	config.Gateways = make(map[string]GatewayConfig)
	config.Relays = make(map[string]RelayConfig)
	config.Users = make(map[string]UserConfig)

	if config.LoadConfig(*configPath) != nil {
		os.Exit(-1)
	}

	dlog.Printf("Config Loaded: %s, %s\n", config.Gateways, config.Users)

	// Create outgoing queue for each relay
	outgoingQueues = make(map[string]chan *Sms)
	for relayName, _ := range config.Relays {
		outgoingQueues[relayName] = make(chan *Sms, BUFFER_SIZE)
	}

	// Create incoming queue
	incomingQueue = make(chan UserMessage, BUFFER_SIZE)

	// Start jobs
	startOutgoingJobs()
	startIncomingJobs()

	// Handle signals
	signal.Notify(sigchan, Reload)
	go SignalHandler()

	//
	go UpdateTime()

	// Bind handlers and start http server
	http.HandleFunc("/send", sendHandler)
	http.HandleFunc("/balance", balanceHandler)
	http.ListenAndServe(*listen, nil)
}
