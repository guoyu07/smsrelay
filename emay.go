package main

import (
	"bytes"
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

type EmayData struct {
	Error   int    `xml:"error"`
	Balance string `xml:"message"`
}

// Mesage received from Emay
type EmayMessage struct {
	Mobile       string `xml:"srctermid"`
	Sendtime     string `xml:"sendTime"`
	Content      string `xml:"msgcontent"`
	AddSerial    string `xml:"addSerial"`
	AddSerialRev string `xml:"addSerialRev"`
}

type EmayIncoming struct {
	Messages []EmayMessage `xml:"message"`
}

type EmayRelay Relay

func (m *EmayMessage) genBody() string {
	keys := []string{"gatewayName", "channel", "mobile", "content", "extData"}
	values := []string{"emay", m.AddSerialRev, m.Mobile, m.Content, ""}
	h := md5.New()
	io.WriteString(h, Encode(keys, values))
	io.WriteString(h, config.Settings.SharedSecret)
	buffer := bytes.NewBufferString("")
	fmt.Fprintf(buffer, "%x", h.Sum(nil))
	keys = append(keys, "_sign")
	values = append(values, buffer.String())
	return Encode(keys, values)
}

func (relay EmayRelay) send(s *Sms) (*http.Response, error) {
	data := url.Values{}
	data.Add("cdkey", relay.Userid)
	data.Add("password", relay.Password)
	data.Add("phone", s.mobile)
	data.Add("message", s.message)
	data.Add("addserial", config.Users[s.from].Extension)

	// dlog.Println(data.Encode())
	return http.PostForm(config.Gateways[relay.Gateway].URL, data)
}

func (relay EmayRelay) receive() (*http.Response, error) {
	data := url.Values{}
	data.Add("cdkey", relay.Userid)
	data.Add("password", relay.Password)

	// dlog.Println(data.Encode())
	return http.PostForm(config.Gateways[relay.Gateway].ReceiveURL, data)
}

func (relay EmayRelay) processSendResult(body []byte) bool {
	return true
}

func (relay EmayRelay) processReceiveResult(body []byte) bool {
	v := EmayIncoming{}
	err := xml.Unmarshal(body, &v)
	if err != nil {
		dlog.Printf("error parsing emay incoming msg: %v", err)
		return false
	}

	for _, msg := range v.Messages {
		if msg.Content != "" {
			incomingQueue <- &msg
			dlog.Printf("Emay received message: %#v\n", msg)
		}
	}

	return true
}

func (relay EmayRelay) checkBalance() string {
	data := url.Values{}
	data.Add("cdkey", relay.Userid)
	data.Add("password", relay.Password)

	resp, err := http.PostForm(config.Gateways[relay.Gateway].BalanceURL, data)

	if err != nil {
		dlog.Println(err)
		return "failed to get balance"
	} else {
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		dlog.Printf("%s\n", body)

		var edata EmayData
		xml.Unmarshal(body, &edata)
		return edata.Balance
	}
}
