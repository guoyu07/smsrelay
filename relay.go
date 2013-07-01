package main

import (
	// "encoding/xml"
	"net/http"
)

type Relay RelayConfig

func GetRelay(name string) SmsRelay {
	if rc, ok := config.Relays[name]; ok {
		switch rc.Gateway {
		case "emay":
			return EmayRelay(rc)
		case "monternet":
			return MontRelay(rc)
		default:
			return nil
		}
	}

	return nil
}

func (relay Relay) send(s *Sms) (*http.Response, error) {
	return nil, nil
}

func (relay Relay) receive() (*http.Response, error) {
	return nil, nil
}

func (relay Relay) processSendResult(body []byte) bool {
	return false
}

func (relay Relay) processReceiveResult(body []byte) bool {
	return false
}

func (relay Relay) checkBalance() string {
	return ""
}
