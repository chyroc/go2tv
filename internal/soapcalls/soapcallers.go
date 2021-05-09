package soapcalls

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type states struct {
	previousState string
	newState      string
	sequence      int
}

var (
	mediaRenderersStates        = make(map[string]*states)
	initialMediaRenderersStates = make(map[string]interface{})
	mu                          sync.Mutex
)

// TVPayload - this is the heard of Go2TV.
type TVPayload struct {
	TransportURL  string
	VideoURL      string
	SubtitlesURL  string
	ControlURL    string
	CallbackURL   string
	CurrentTimers map[string]*time.Timer
}

func (p *TVPayload) setAVTransportSoapCall() error {
	parsedURLtransport, err := url.Parse(p.TransportURL)
	if err != nil {
		return err
	}

	xml, err := setAVTransportSoapBuild(p.VideoURL, p.SubtitlesURL)
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", parsedURLtransport.String(), bytes.NewReader(xml))
	if err != nil {
		return err
	}

	headers := http.Header{
		"SOAPAction":   []string{`"urn:schemas-upnp-org:service:AVTransport:1#SetAVTransportURI"`},
		"content-type": []string{"text/xml"},
		"charset":      []string{"utf-8"},
		"Connection":   []string{"close"},
	}
	req.Header = headers

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}

// PlayStopSoapCall - Build and call the play soap call.
func (p *TVPayload) playStopPauseSoapCall(action string) error {
	parsedURLtransport, err := url.Parse(p.TransportURL)
	if err != nil {
		return err
	}

	var xml []byte
	if action == "Play" {
		xml, err = playSoapBuild()
	}
	if action == "Stop" {
		xml, err = stopSoapBuild()
	}
	if action == "Pause" {
		xml, err = pauseSoapBuild()
	}
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", parsedURLtransport.String(), bytes.NewReader(xml))
	if err != nil {
		return err
	}

	headers := http.Header{
		"SOAPAction":   []string{`"urn:schemas-upnp-org:service:AVTransport:1#` + action + `"`},
		"content-type": []string{"text/xml"},
		"charset":      []string{"utf-8"},
		"Connection":   []string{"close"},
	}
	req.Header = headers

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}

// SubscribeSoapCall - Subscribe to a media renderer
// If we explicitly pass the uuid, then we refresh it instead.
func (p *TVPayload) SubscribeSoapCall(uuidInput string) error {
	delete(p.CurrentTimers, uuidInput)

	parsedURLcontrol, err := url.Parse(p.ControlURL)
	if err != nil {
		return err
	}

	parsedURLcallback, err := url.Parse(p.CallbackURL)
	if err != nil {
		return err
	}

	client := &http.Client{}

	req, err := http.NewRequest("SUBSCRIBE", parsedURLcontrol.String(), nil)
	if err != nil {
		return err
	}

	var headers http.Header
	if uuidInput == "" {
		headers = http.Header{
			"USER-AGENT": []string{runtime.GOOS + "  UPnP/1.1 " + "Go2TV"},
			"CALLBACK":   []string{"<" + parsedURLcallback.String() + ">"},
			"NT":         []string{"upnp:event"},
			"TIMEOUT":    []string{"Second-300"},
			"Connection": []string{"close"},
		}
	} else {
		headers = http.Header{
			"SID":        []string{"uuid:" + uuidInput},
			"TIMEOUT":    []string{"Second-300"},
			"Connection": []string{"close"},
		}
	}
	req.Header = headers
	req.Header.Del("User-Agent")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var uuid string

	if resp.Status != "200 OK" {
		if uuidInput != "" {
			// We're calling the unsubscribe method to make sure
			// we clean up any remaining states for the specific
			// uuid. The actual UNSUBSCRIBE request to the media
			// renderer may still fail with error 412, but it's fine.
			p.UnsubscribeSoapCall(uuid)
		}
		return nil
	}

	if len(resp.Header["Sid"]) > 0 {
		uuid = resp.Header["Sid"][0]
		uuid = strings.TrimLeft(uuid, "[")
		uuid = strings.TrimLeft(uuid, "]")
		uuid = strings.TrimPrefix(uuid, "uuid:")
	} else {
		// This should be an impossible case
		return nil
	}

	// We don't really need to initialize or set
	// the State if we're just refreshing the uuid.
	if uuidInput == "" {
		CreateMRstate(uuid)
	}

	timeoutReply := "300"
	if len(resp.Header["Timeout"]) > 0 {
		timeoutReply = strings.TrimLeft(resp.Header["Timeout"][0], "Second-")
	}

	p.RefreshLoopUUIDSoapCall(uuid, timeoutReply)

	return nil
}

// UnsubscribeSoapCall - exported that as we use
// it for the callback stuff in the httphandlers package.
func (p *TVPayload) UnsubscribeSoapCall(uuid string) error {
	DeleteMRstate(uuid)

	parsedURLcontrol, err := url.Parse(p.ControlURL)
	if err != nil {
		return err
	}

	client := &http.Client{}

	req, err := http.NewRequest("UNSUBSCRIBE", parsedURLcontrol.String(), nil)
	if err != nil {
		return err
	}

	headers := http.Header{
		"SID":        []string{"uuid:" + uuid},
		"Connection": []string{"close"},
	}

	req.Header = headers
	req.Header.Del("User-Agent")

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}

// RefreshLoopUUIDSoapCall - Refresh the UUID.
func (p *TVPayload) RefreshLoopUUIDSoapCall(uuid, timeout string) error {
	triggerTime := 5
	timeoutInt, err := strconv.Atoi(timeout)
	if err != nil {
		return err
	}

	// Refresh token after after Timeout / 2 seconds.
	if timeoutInt > 20 {
		triggerTime = timeoutInt / 5
	}
	triggerTimefunc := time.Duration(triggerTime) * time.Second

	// We're doing this as time.AfterFunc can't handle
	// function arguments.
	f := p.refreshLoopUUIDAsyncSoapCall(uuid)
	timer := time.AfterFunc(triggerTimefunc, f)
	p.CurrentTimers[uuid] = timer

	return nil
}

func (p *TVPayload) refreshLoopUUIDAsyncSoapCall(uuid string) func() {
	return func() {
		p.SubscribeSoapCall(uuid)
	}
}

// SendtoTV - Send to TV.
func (p *TVPayload) SendtoTV(action string) error {
	if action == "Play1" {
		if err := p.SubscribeSoapCall(""); err != nil {
			return err
		}
		if err := p.setAVTransportSoapCall(); err != nil {
			return err
		}
		action = "Play"
	}

	if action == "Stop" {
		// Cleaning up all uuids on force stop.
		for uuids := range mediaRenderersStates {
			p.UnsubscribeSoapCall(uuids)
		}

		// Clear timers on Stop to avoid errors responses
		// from the media renderers. If we don't clear those, we
		// might receive a "412 Precondition Failed" error.
		for uuid, timer := range p.CurrentTimers {
			timer.Stop()
			delete(p.CurrentTimers, uuid)
		}
	}
	if err := p.playStopPauseSoapCall(action); err != nil {
		return err
	}

	return nil
}

// UpdateMRstate - Update the mediaRenderersStates map
// with the state. Return true or false to verify that
// the actual update took place.
func UpdateMRstate(previous, new, uuid string) bool {
	mu.Lock()
	defer mu.Unlock()
	// If the uuid is not one of the UUIDs we stored in
	// soapcalls.InitialMediaRenderersStates it means that
	// probably it expired and there is not much we can do
	// with it. Trying to send an unsubscribe for those will
	// probably result in a 412 error as per the upnpn documentation
	// http://upnp.org/specs/arch/UPnP-arch-DeviceArchitecture-v1.1.pdf
	// (page 94).
	if initialMediaRenderersStates[uuid] == true {
		mediaRenderersStates[uuid].previousState = previous
		mediaRenderersStates[uuid].newState = new
		mediaRenderersStates[uuid].sequence++
		return true
	}

	return false
}

// CreateMRstate .
func CreateMRstate(uuid string) {
	mu.Lock()
	defer mu.Unlock()
	initialMediaRenderersStates[uuid] = true
	mediaRenderersStates[uuid] = &states{
		previousState: "",
		newState:      "",
		sequence:      0,
	}
}

// DeleteMRstate .
func DeleteMRstate(uuid string) {
	mu.Lock()
	defer mu.Unlock()
	delete(initialMediaRenderersStates, uuid)
	delete(mediaRenderersStates, uuid)
}

// IncreaseSequence .
func IncreaseSequence(uuid string) {
	mu.Lock()
	defer mu.Unlock()
	mediaRenderersStates[uuid].sequence++
}

// GetSequence .
func GetSequence(uuid string) (int, error) {
	if initialMediaRenderersStates[uuid] == true {
		return mediaRenderersStates[uuid].sequence, nil
	}

	return -1, errors.New("zombie callbacks, we should ignore those")
}
