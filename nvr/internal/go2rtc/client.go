// Package go2rtc provides a thin HTTP client for the go2rtc relay daemon.
// go2rtc ingests RTSP from cameras and exposes browser-friendly WebRTC/HLS
// output without transcoding (H.264 passthrough).
package go2rtc

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a local go2rtc instance via its REST API.
// All registration methods are best-effort; errors are logged but not returned
// so a degraded go2rtc cannot crash the NVR.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient returns a Client. If baseURL is empty the client is
// disabled and all methods are no-ops.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Enabled reports whether a go2rtc address was configured.
func (c *Client) Enabled() bool { return c.baseURL != "" }

// RegisterCameraStreams upserts all RTSP stream URIs for a camera in go2rtc.
// Stream names: cam{id}_primary, cam{id}_sub, cam{id}_stream3, …
func (c *Client) RegisterCameraStreams(cameraID int64, username, password string, streamURIs []string) {
	for i, uri := range streamURIs {
		name := streamName(cameraID, i)
		withCreds := injectCreds(uri, username, password)
		if err := c.putStream(name, withCreds); err != nil {
			log.Printf("go2rtc: register %s: %v", name, err)
		}
	}
}

// UnregisterCameraStreams removes all streams for a camera (best-effort).
func (c *Client) UnregisterCameraStreams(cameraID int64) {
	for i := range streamLabels {
		c.deleteStream(streamName(cameraID, i))
	}
}

// WebRTC registers the given RTSP URI as streamIdx (0=primary, 1=sub) in
// go2rtc, then proxies a WebRTC SDP offer/answer exchange.
// Returns the SDP answer on success.
func (c *Client) WebRTC(cameraID int64, streamIdx int, username, password, rtspURI, offerSDP string) (string, error) {
	name := streamName(cameraID, streamIdx)
	if err := c.putStream(name, injectCreds(rtspURI, username, password)); err != nil {
		return "", fmt.Errorf("register stream: %w", err)
	}
	return c.webrtcOffer(name, offerSDP)
}

// ---------------------------------------------------------------------------
// internal helpers

var streamLabels = []string{"primary", "sub", "stream3"}

func streamName(cameraID int64, idx int) string {
	if idx < len(streamLabels) {
		return fmt.Sprintf("cam%d_%s", cameraID, streamLabels[idx])
	}
	return fmt.Sprintf("cam%d_stream%d", cameraID, idx)
}

// injectCreds embeds username:password into an RTSP (or HTTP) URL.
func injectCreds(rawURL, username, password string) string {
	if username == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.User = url.UserPassword(username, password)
	return u.String()
}

// putStream calls PUT /api/streams?name=<n>&src=<sourceURL> to upsert a stream.
func (c *Client) putStream(name, sourceURL string) error {
	endpoint := c.baseURL + "/api/streams?name=" + url.QueryEscape(name) + "&src=" + url.QueryEscape(sourceURL)
	req, err := http.NewRequest(http.MethodPut, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) deleteStream(name string) {
	endpoint := c.baseURL + "/api/streams?name=" + url.QueryEscape(name)
	req, _ := http.NewRequest(http.MethodDelete, endpoint, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// webrtcOffer posts an SDP offer to go2rtc and returns the SDP answer.
// go2rtc endpoint: POST /api/webrtc?src=<stream_name>
// Content-Type: application/sdp
// Uses a longer timeout than other calls because go2rtc must connect to the
// camera RTSP source on the first viewer request (lazy connection).
func (c *Client) webrtcOffer(name, offerSDP string) (string, error) {
	endpoint := c.baseURL + "/api/webrtc?src=" + url.QueryEscape(name)
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(offerSDP))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/sdp")

	// Longer timeout: go2rtc needs to connect to the camera RTSP + do ICE negotiation.
	longClient := &http.Client{Timeout: 20 * time.Second}
	resp, err := longClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("go2rtc WebRTC %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}
