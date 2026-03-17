package onvif

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

type ProbeResult struct {
	Manufacturer string
	Model        string
	Firmware     string
	HasONVIF     bool
	HasPTZ       bool
	HasMotion    bool
	HasInfrared  bool
	Resolutions   []string
	StreamURIs    []string
	SnapshotURIs  []string
}

type deviceInfoResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			Manufacturer    string `xml:"Manufacturer"`
			Model           string `xml:"Model"`
			FirmwareVersion string `xml:"FirmwareVersion"`
		} `xml:"GetDeviceInformationResponse"`
	} `xml:"Body"`
}

type capabilitiesResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			Capabilities struct {
				Events struct {
					XAddr string `xml:"XAddr"`
				} `xml:"Events"`
				PTZ *struct {
					XAddr string `xml:"XAddr"`
				} `xml:"PTZ"`
				Media struct {
					XAddr string `xml:"XAddr"`
				} `xml:"Media"`
				Imaging *struct {
					XAddr string `xml:"XAddr"`
				} `xml:"Imaging"`
			} `xml:"Capabilities"`
		} `xml:"GetCapabilitiesResponse"`
	} `xml:"Body"`
}

type profilesResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			Profiles []struct {
				Token string `xml:"token,attr"`
				Name  string `xml:"Name"`
				Video *struct {
					Resolution struct {
						Width  int `xml:"Width"`
						Height int `xml:"Height"`
					} `xml:"Resolution"`
				} `xml:"VideoEncoderConfiguration"`
			} `xml:"Profiles"`
		} `xml:"GetProfilesResponse"`
	} `xml:"Body"`
}

type streamURIResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			MediaURI struct {
				URI string `xml:"Uri"`
			} `xml:"MediaUri"`
		} `xml:"GetStreamUriResponse"`
	} `xml:"Body"`
}

type snapshotURIResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			MediaURI struct {
				URI string `xml:"Uri"`
			} `xml:"MediaUri"`
		} `xml:"GetSnapshotUriResponse"`
	} `xml:"Body"`
}

// Probe connects to a camera via ONVIF and discovers its capabilities.
func Probe(ip string, port int, onvifPath, username, password string) (*ProbeResult, error) {
	baseURL := fmt.Sprintf("http://%s:%d%s", ip, port, onvifPath)

	// Test basic connectivity first
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s:%d: %w", ip, port, err)
	}
	conn.Close()

	result := &ProbeResult{}

	// 1. GetDeviceInformation
	var devInfo deviceInfoResp
	if err := soapCallAuth(baseURL,
		cleanEnvelope(`<tds:GetDeviceInformation xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>`),
		"http://www.onvif.org/ver10/device/wsdl/GetDeviceInformation",
		username, password, &devInfo); err == nil {
		result.HasONVIF = true
		result.Manufacturer = devInfo.Body.Resp.Manufacturer
		result.Model = devInfo.Body.Resp.Model
		result.Firmware = devInfo.Body.Resp.FirmwareVersion
	} else {
		return result, fmt.Errorf("ONVIF probe: %w", err)
	}

	// 2. GetCapabilities
	var caps capabilitiesResp
	if err := soapCallAuth(baseURL,
		cleanEnvelope(`<tds:GetCapabilities xmlns:tds="http://www.onvif.org/ver10/device/wsdl"><tds:Category>All</tds:Category></tds:GetCapabilities>`),
		"http://www.onvif.org/ver10/device/wsdl/GetCapabilities",
		username, password, &caps); err == nil {
		if caps.Body.Resp.Capabilities.PTZ != nil {
			result.HasPTZ = true
		}
		if caps.Body.Resp.Capabilities.Events.XAddr != "" {
			result.HasMotion = true // has events = likely has motion
		}
		if caps.Body.Resp.Capabilities.Imaging != nil {
			result.HasInfrared = true // has imaging service = likely has IR
		}
	}

	// 3. GetProfiles (for resolutions and stream URIs)
	mediaURL := baseURL
	if caps.Body.Resp.Capabilities.Media.XAddr != "" {
		mediaURL = caps.Body.Resp.Capabilities.Media.XAddr
	}

	var profiles profilesResp
	if err := soapCallAuth(mediaURL,
		cleanEnvelope(`<trt:GetProfiles xmlns:trt="http://www.onvif.org/ver10/media/wsdl"/>`),
		"http://www.onvif.org/ver10/media/wsdl/GetProfiles",
		username, password, &profiles); err == nil {
		for _, p := range profiles.Body.Resp.Profiles {
			if p.Video != nil {
				res := fmt.Sprintf("%dx%d", p.Video.Resolution.Width, p.Video.Resolution.Height)
				result.Resolutions = append(result.Resolutions, res)
			}

			// 4. GetStreamUri for each profile
			streamBody := fmt.Sprintf(
				`<trt:GetStreamUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl"><trt:StreamSetup><tt:Stream xmlns:tt="http://www.onvif.org/ver10/schema">RTP-Unicast</tt:Stream><tt:Transport xmlns:tt="http://www.onvif.org/ver10/schema"><tt:Protocol>RTSP</tt:Protocol></tt:Transport></trt:StreamSetup><trt:ProfileToken>%s</trt:ProfileToken></trt:GetStreamUri>`,
				p.Token)
			var streamResp streamURIResp
			if err := soapCallAuth(mediaURL, cleanEnvelope(streamBody),
				"http://www.onvif.org/ver10/media/wsdl/GetStreamUri",
				username, password, &streamResp); err == nil {
				if uri := streamResp.Body.Resp.MediaURI.URI; uri != "" {
					result.StreamURIs = append(result.StreamURIs, uri)
				}
			}

			// 5. GetSnapshotUri for each profile
			snapBody := fmt.Sprintf(
				`<trt:GetSnapshotUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl"><trt:ProfileToken>%s</trt:ProfileToken></trt:GetSnapshotUri>`,
				p.Token)
			var snapResp snapshotURIResp
			if err := soapCallAuth(mediaURL, cleanEnvelope(snapBody),
				"http://www.onvif.org/ver10/media/wsdl/GetSnapshotUri",
				username, password, &snapResp); err == nil {
				if uri := snapResp.Body.Resp.MediaURI.URI; uri != "" {
					result.SnapshotURIs = append(result.SnapshotURIs, uri)
				}
			}
		}
	}

	return result, nil
}

// TestConnection does a quick reachability check on HTTP and RTSP ports.
func TestConnection(ip string, httpPort, rtspPort int) error {
	for _, p := range []int{httpPort, rtspPort} {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, p), 3*time.Second)
		if err != nil {
			return fmt.Errorf("port %d unreachable: %w", p, err)
		}
		conn.Close()
	}
	return nil
}

// cleanEnvelope wraps a SOAP body element in a minimal SOAP 1.2 envelope (no WS-Security).
func cleanEnvelope(innerBody string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body>%s</s:Body></s:Envelope>`, innerBody)
}

// soapCallAuth sends a SOAP 1.2 request with the required action in Content-Type.
// On 401 Digest challenge it retries with HTTP Digest auth.
func soapCallAuth(rawURL, body, action, username, password string, result any) error {
	ct := `application/soap+xml; charset=utf-8`
	if action != "" {
		ct = fmt.Sprintf(`application/soap+xml; charset=utf-8; action="%s"`, action)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(rawURL, ct, strings.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 401 with Digest challenge — retry with HTTP Digest auth
	if resp.StatusCode == 401 && username != "" {
		challenge := resp.Header.Get("WWW-Authenticate")
		if strings.HasPrefix(challenge, "Digest ") {
			return soapCallDigest(client, rawURL, body, ct, username, password, challenge, result)
		}
		return fmt.Errorf("authentication failed (no Digest challenge)")
	}

	if resp.StatusCode != 200 {
		if bytes.Contains(data, []byte("NotAuthorized")) {
			if i := bytes.Index(data, []byte("<env:Text")); i >= 0 {
				if j := bytes.Index(data[i:], []byte(">")); j >= 0 {
					if k := bytes.Index(data[i+j:], []byte("</env:Text>")); k >= 0 {
						return fmt.Errorf("not authorized: %s", string(data[i+j+1:i+j+k]))
					}
				}
			}
			return fmt.Errorf("not authorized")
		}
		return fmt.Errorf("SOAP error (HTTP %d): %s", resp.StatusCode, truncate(data, 200))
	}

	return xml.Unmarshal(data, result)
}

func truncate(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n]) + "..."
}

func soapCallDigest(client *http.Client, rawURL, body, ct, username, password, challenge string, result any) error {
	params := parseDigestChallenge(challenge)
	realm := params["realm"]
	nonce := params["nonce"]
	qop := params["qop"]

	// Extract URI path from full URL for Digest calculation
	uri := rawURL
	if i := strings.Index(rawURL, "://"); i >= 0 {
		rest := rawURL[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			uri = rest[j:]
		}
	}

	// Generate cnonce and compute digest
	cnonceBytes := make([]byte, 8)
	rand.Read(cnonceBytes)
	cnonce := hex.EncodeToString(cnonceBytes)
	nc := "00000001"

	ha1 := md5hex(username + ":" + realm + ":" + password)
	ha2 := md5hex("POST:" + uri)

	var response string
	if strings.Contains(qop, "auth") {
		response = md5hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":auth:" + ha2)
	} else {
		response = md5hex(ha1 + ":" + nonce + ":" + ha2)
	}

	authHeader := fmt.Sprintf(
		`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", nc=%s, cnonce="%s", qop=auth`,
		username, realm, nonce, uri, response, nc, cnonce,
	)

	req, err := http.NewRequest("POST", rawURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", authHeader)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("digest auth failed (HTTP %d): %s", resp.StatusCode, truncate(data, 200))
	}

	return xml.Unmarshal(data, result)
}

func parseDigestChallenge(header string) map[string]string {
	params := map[string]string{}
	// Strip "Digest " prefix
	s := strings.TrimPrefix(header, "Digest ")
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if i := strings.IndexByte(part, '='); i > 0 {
			key := strings.TrimSpace(part[:i])
			val := strings.TrimSpace(part[i+1:])
			val = strings.Trim(val, `"`)
			params[key] = val
		}
	}
	return params
}

func md5hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
