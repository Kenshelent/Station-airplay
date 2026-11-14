// A one-shot Luna binary plugin. It never starts/stops the AirPlay receiver.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"station-airplay/internal/panelrpc"
)

func main() {
	if len(os.Args) != 2 || len(os.Args[1]) > panelrpc.MaxWire {
		os.Exit(2)
	}
	var invocation struct {
		Request panelrpc.Request `json:"request"`
		Attempt string           `json:"attempt"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(os.Args[1]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&invocation) != nil || decoder.Decode(new(any)) != io.EOF || !panelrpc.Valid(invocation.Request) || len(invocation.Attempt) != 32 {
		os.Exit(2)
	}
	request := invocation.Request
	wire, _ := json.Marshal(request)
	reply := panelrpc.Reply{ID: request.ID, Epoch: request.Epoch, Status: 503,
		Body: json.RawMessage(`{"error":"AirPlay unavailable; start a compatible receiver"}`)}
	client := &http.Client{Timeout: 2500 * time.Millisecond, Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", "/run/station-airplay/control.sock")
		},
	}}
	if response, err := client.Post("http://localhost/rpc", "application/json", bytes.NewReader(wire)); err == nil {
		data, err := io.ReadAll(io.LimitReader(response.Body, panelrpc.MaxWire+1))
		response.Body.Close()
		var received panelrpc.Reply
		if err == nil && len(data) <= panelrpc.MaxWire && json.Unmarshal(data, &received) == nil && received.ID == request.ID && json.Valid(received.Body) {
			reply = received
		}
	}
	data, _ := json.Marshal(struct {
		panelrpc.Reply
		Attempt string `json:"attempt"`
	}{reply, invocation.Attempt})
	// Frame with a leading newline so a killed earlier writer's partial line
	// cannot consume a complete new result. No credentials enter these logs.
	fmt.Printf("\nAP1 %s\n", base64.StdEncoding.EncodeToString(data))
}
