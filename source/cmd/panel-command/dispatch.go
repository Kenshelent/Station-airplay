package main

import "encoding/json"

// startPackageUpdate handles the panel's update button. It is a separate,
// deliberately narrow payload so the receiver RPC stays unchanged.
func startPackageUpdate(raw string) bool {
	var request struct {
		Update  bool   `json:"update"`
		Attempt string `json:"attempt"`
	}
	if json.Unmarshal([]byte(raw), &request) != nil || !request.Update || !validAttempt(request.Attempt) {
		return false
	}
	startUpdate()
	return true
}

func validAttempt(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
