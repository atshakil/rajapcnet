package onvif

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// MotionNotification is a normalised motion event from an ONVIF camera.
type MotionNotification struct {
	IsMotion    bool
	Active      bool
	Topic       string
	RuleName    string
	SourceToken string
	CameraTime  time.Time // zero if unavailable
}

// PullPointSession manages an ONVIF PullPoint event subscription.
type PullPointSession struct {
	pullURL     string
	renewBefore time.Time
	username    string
	password    string
}

// ---------------------------------------------------------------------------
// XML types for event responses

type createPullPointResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			Reference struct {
				Address string `xml:"Address"`
			} `xml:"SubscriptionReference"`
			CurrentTime     string `xml:"CurrentTime"`
			TerminationTime string `xml:"TerminationTime"`
		} `xml:"CreatePullPointSubscriptionResponse"`
	} `xml:"Body"`
}

type pullMessagesResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			CurrentTime     string         `xml:"CurrentTime"`
			TerminationTime string         `xml:"TerminationTime"`
			Notifications   []notification `xml:"NotificationMessage"`
		} `xml:"PullMessagesResponse"`
	} `xml:"Body"`
}

type notification struct {
	Topic   topicEl `xml:"Topic"`
	Message struct {
		Inner innerMessage `xml:"Message"` // tt:Message inside wsnt:Message
	} `xml:"Message"` // wsnt:Message
}

type topicEl struct {
	Value string `xml:",chardata"`
}

type innerMessage struct {
	UtcTime string `xml:"UtcTime,attr"`
	Source  struct {
		Items []simpleItem `xml:"SimpleItem"`
	} `xml:"Source"`
	Data struct {
		Items []simpleItem `xml:"SimpleItem"`
	} `xml:"Data"`
}

type simpleItem struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:"Value,attr"`
}

// ---------------------------------------------------------------------------
// Public API

// CreatePullPoint discovers the Events service on the camera and creates a
// PullPoint subscription. Returns a session for pulling messages.
func CreatePullPoint(ip string, port int, onvifPath, username, password string) (*PullPointSession, error) {
	baseURL := fmt.Sprintf("http://%s:%d%s", ip, port, onvifPath)

	// Discover Events XAddr
	var caps capabilitiesResp
	if err := soapCallAuth(baseURL,
		`<tds:GetCapabilities xmlns:tds="http://www.onvif.org/ver10/device/wsdl"><tds:Category>All</tds:Category></tds:GetCapabilities>`,
		"http://www.onvif.org/ver10/device/wsdl/GetCapabilities",
		username, password, &caps); err != nil {
		return nil, fmt.Errorf("GetCapabilities: %w", err)
	}
	eventsXAddr := caps.Body.Resp.Capabilities.Events.XAddr
	if eventsXAddr == "" {
		return nil, fmt.Errorf("camera does not advertise Events service")
	}

	// Create PullPoint subscription
	var ppr createPullPointResp
	if err := soapCallAuth(eventsXAddr,
		`<tev:CreatePullPointSubscription xmlns:tev="http://www.onvif.org/ver10/events/wsdl"><tev:InitialTerminationTime>PT60S</tev:InitialTerminationTime></tev:CreatePullPointSubscription>`,
		"http://www.onvif.org/ver10/events/wsdl/CreatePullPointSubscription",
		username, password, &ppr); err != nil {
		return nil, fmt.Errorf("CreatePullPointSubscription: %w", err)
	}

	pullURL := strings.TrimSpace(ppr.Body.Resp.Reference.Address)
	if pullURL == "" {
		return nil, fmt.Errorf("empty PullPoint address in subscription response")
	}

	renewBefore := time.Now().Add(50 * time.Second) // conservative default
	if t, err := time.Parse(time.RFC3339, ppr.Body.Resp.TerminationTime); err == nil {
		renewBefore = t.Add(-10 * time.Second)
	}

	return &PullPointSession{
		pullURL:     pullURL,
		renewBefore: renewBefore,
		username:    username,
		password:    password,
	}, nil
}

// Pull sends a PullMessages SOAP call and returns normalised motion notifications.
// Blocks up to ~10 s waiting for events. Returns empty slice (not error) when no events arrive.
func (s *PullPointSession) Pull(ctx context.Context) ([]MotionNotification, error) {
	var resp pullMessagesResp
	if err := soapCallAuthTimeout(s.pullURL,
		`<tev:PullMessages xmlns:tev="http://www.onvif.org/ver10/events/wsdl"><tev:Timeout>PT10S</tev:Timeout><tev:MessageLimit>100</tev:MessageLimit></tev:PullMessages>`,
		"http://www.onvif.org/ver10/events/wsdl/PullMessages",
		s.username, s.password,
		20*time.Second, // HTTP timeout > PullMessages timeout
		&resp); err != nil {
		return nil, err
	}

	// Update renewal time from response
	if t, err := time.Parse(time.RFC3339, resp.Body.Resp.TerminationTime); err == nil {
		s.renewBefore = t.Add(-10 * time.Second)
	}

	var out []MotionNotification
	for _, n := range resp.Body.Resp.Notifications {
		topic := strings.TrimSpace(n.Topic.Value)
		if !isMotionTopic(topic) {
			continue
		}

		isMot, active := parseMotionData(n.Message.Inner.Data.Items)
		if !isMot {
			continue
		}

		mn := MotionNotification{
			IsMotion: true,
			Active:   active,
			Topic:    topic,
		}

		// Extract source metadata
		for _, item := range n.Message.Inner.Source.Items {
			switch strings.ToLower(item.Name) {
			case "rule":
				mn.RuleName = item.Value
			case "videosourceconfigurationtoken":
				mn.SourceToken = item.Value
			}
		}

		// Parse camera timestamp
		if n.Message.Inner.UtcTime != "" {
			if t, err := time.Parse(time.RFC3339, n.Message.Inner.UtcTime); err == nil {
				mn.CameraTime = t
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", n.Message.Inner.UtcTime); err == nil {
				mn.CameraTime = t
			}
		}

		out = append(out, mn)
	}
	return out, nil
}

// NeedsRenew returns true if the subscription should be renewed soon.
func (s *PullPointSession) NeedsRenew() bool {
	return time.Now().After(s.renewBefore)
}

// Renew extends the subscription lifetime.
func (s *PullPointSession) Renew() error {
	var dummy struct{}
	if err := soapCallAuth(s.pullURL,
		`<wsnt:Renew xmlns:wsnt="http://docs.oasis-open.org/wsn/bw-2"><wsnt:TerminationTime>PT60S</wsnt:TerminationTime></wsnt:Renew>`,
		"http://docs.oasis-open.org/wsn/bw-2/SubscriptionManager/RenewRequest",
		s.username, s.password, &dummy); err != nil {
		return err
	}
	s.renewBefore = time.Now().Add(50 * time.Second)
	return nil
}

// Unsubscribe terminates the PullPoint subscription (best-effort).
func (s *PullPointSession) Unsubscribe() {
	var dummy struct{}
	_ = soapCallAuth(s.pullURL,
		`<wsnt:Unsubscribe xmlns:wsnt="http://docs.oasis-open.org/wsn/bw-2"/>`,
		"http://docs.oasis-open.org/wsn/bw-2/SubscriptionManager/UnsubscribeRequest",
		s.username, s.password, &dummy)
}

// ---------------------------------------------------------------------------
// Helpers

func isMotionTopic(topic string) bool {
	t := strings.ToLower(topic)
	return strings.Contains(t, "motion") || strings.Contains(t, "motionalarm")
}

func parseMotionData(items []simpleItem) (isMotion bool, active bool) {
	for _, item := range items {
		name := strings.ToLower(item.Name)
		if name == "ismotion" || name == "state" || name == "isactive" {
			val := strings.ToLower(strings.TrimSpace(item.Value))
			return true, val == "true" || val == "1"
		}
	}
	return false, false
}
