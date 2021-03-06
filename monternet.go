package main

import (
	"bytes"
	"crypto/md5"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type MontRelay Relay

// Message received from Monternet
type MontMessage string

// Incoming messages received from Monternet
type MontIncoming struct {
	Result []MontMessage `xml:"string"`
}

func (m *MontMessage) genBody() string {
	keys := []string{"gatewayName", "channel", "mobile", "content", "extData"}

	reader := csv.NewReader(strings.NewReader(string(*m)))
	record, err := reader.Read()
	if err != nil && err != io.EOF {
		dlog.Println("Error occurred when parsing MontMessage:", err)
		return ""
	}

	length := len(record)
	// var channel, mobile, content string

	if length != 6 {
		dlog.Printf("Failed to parse MontMessage: length: %d, msg: %s\n", length, m)
		return ""
	}

	channel := ""
	for _, number := range config.Gateways["monternet"].CallerNumbers {
		if strings.HasPrefix(record[3], number) {
			channel = strings.TrimPrefix(record[3], number)
		}
	}

	values := []string{
		"monternet",
		channel,
		record[2],
		record[5],
		""}
	h := md5.New()
	io.WriteString(h, Encode(keys, values))
	io.WriteString(h, config.Settings.SharedSecret)
	buffer := bytes.NewBufferString("")
	fmt.Fprintf(buffer, "%x", h.Sum(nil))
	keys = append(keys, "_sign")
	values = append(values, buffer.String())
	return Encode(keys, values)
}

// NB: Cannot use PostForm here because montnets webservice requires fixed ordering
func (relay MontRelay) send(s *Sms) (*http.Response, error) {
	keys := []string{"userId", "password", "pszMobis", "pszMsg", "iMobiCount", "pszSubPort"}
	values := []string{relay.Userid, relay.Password, s.mobile, url.QueryEscape(s.message), strconv.Itoa(s.count), config.Users[s.from].Extension}

	return http.Post(config.Gateways[relay.Gateway].URL,
		"application/x-www-form-urlencoded",
		strings.NewReader(Encode(keys, values)))
}

func (relay MontRelay) receive() (*http.Response, error) {
	keys := []string{"userId", "password"}
	values := []string{relay.Userid, relay.Password}

	return http.Post(config.Gateways[relay.Gateway].ReceiveURL,
		"application/x-www-form-urlencoded",
		strings.NewReader(Encode(keys, values)))
}

func (relay MontRelay) processSendResult(body []byte) bool {
	return true
}

func (relay MontRelay) processReceiveResult(body []byte) bool {
	v := MontIncoming{}
	err := xml.Unmarshal(body, &v)
	if err != nil {
		dlog.Println("Failed to unmarshal monternet incoming msg:", err)
		return false
	}

	// dlog.Printf("Result: %#v\n", v.Result)

	for _, msg := range v.Result {
		incomingQueue <- &msg
		dlog.Println("Mont received message:", msg)
	}

	return true
}

func (relay MontRelay) checkBalance() string {
	keys := []string{"userId", "password"}
	values := []string{relay.Userid, relay.Password}

	resp, err := http.Post(config.Gateways[relay.Gateway].BalanceURL,
		"application/x-www-form-urlencoded",
		strings.NewReader(Encode(keys, values)))

	if err != nil {
		dlog.Println(err)
		return "failed to get balance"
	} else {
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		dlog.Printf("%s\n", body)

		var balance string
		xml.Unmarshal(body, &balance)
		return balance
	}
}
