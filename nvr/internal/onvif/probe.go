package onvif

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
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
	Resolutions  []string
	StreamURIs   []string
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
	devReq := soapEnvelope(username, password, `
		<tds:GetDeviceInformation xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>
	`)
	var devInfo deviceInfoResp
	if err := soapCall(baseURL, devReq, &devInfo); err == nil {
		result.HasONVIF = true
		result.Manufacturer = devInfo.Body.Resp.Manufacturer
		result.Model = devInfo.Body.Resp.Model
		result.Firmware = devInfo.Body.Resp.FirmwareVersion
	} else {
		// Not ONVIF capable or bad credentials
		return result, nil
	}

	// 2. GetCapabilities
	capReq := soapEnvelope(username, password, `
		<tds:GetCapabilities xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
			<tds:Category>All</tds:Category>
		</tds:GetCapabilities>
	`)
	var caps capabilitiesResp
	if err := soapCall(baseURL, capReq, &caps); err == nil {
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

	profReq := soapEnvelope(username, password, `
		<trt:GetProfiles xmlns:trt="http://www.onvif.org/ver10/media/wsdl"/>
	`)
	var profiles profilesResp
	if err := soapCall(mediaURL, profReq, &profiles); err == nil {
		for _, p := range profiles.Body.Resp.Profiles {
			if p.Video != nil {
				res := fmt.Sprintf("%dx%d", p.Video.Resolution.Width, p.Video.Resolution.Height)
				result.Resolutions = append(result.Resolutions, res)
			}

			// 4. GetStreamUri for each profile
			streamReq := soapEnvelope(username, password, fmt.Sprintf(`
				<trt:GetStreamUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
					<trt:StreamSetup>
						<tt:Stream xmlns:tt="http://www.onvif.org/ver10/schema">RTP-Unicast</tt:Stream>
						<tt:Transport xmlns:tt="http://www.onvif.org/ver10/schema">
							<tt:Protocol>RTSP</tt:Protocol>
						</tt:Transport>
					</trt:StreamSetup>
					<trt:ProfileToken>%s</trt:ProfileToken>
				</trt:GetStreamUri>
			`, p.Token))
			var streamResp streamURIResp
			if err := soapCall(mediaURL, streamReq, &streamResp); err == nil {
				if uri := streamResp.Body.Resp.MediaURI.URI; uri != "" {
					result.StreamURIs = append(result.StreamURIs, uri)
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

func soapEnvelope(username, password, body string) string {
	header := ""
	if username != "" {
		header = wsseHeader(username, password)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
	<s:Header>%s</s:Header>
	<s:Body>%s</s:Body>
</s:Envelope>`, header, body)
}

func wsseHeader(username, password string) string {
	nonce := make([]byte, 16)
	rand.Read(nonce)
	created := time.Now().UTC().Format(time.RFC3339Nano)

	h := sha1.New()
	h.Write(nonce)
	h.Write([]byte(created))
	h.Write([]byte(password))
	digest := h.Sum(nil)

	return fmt.Sprintf(`
		<Security xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
			<UsernameToken>
				<Username>%s</Username>
				<Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</Password>
				<Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</Nonce>
				<Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">%s</Created>
			</UsernameToken>
		</Security>`,
		username,
		base64.StdEncoding.EncodeToString(digest),
		base64.StdEncoding.EncodeToString(nonce),
		created,
	)
}

func soapCall(url, body string, result any) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/soap+xml; charset=utf-8", strings.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		// Check for common SOAP faults
		if bytes.Contains(data, []byte("NotAuthorized")) || bytes.Contains(data, []byte("401")) {
			return fmt.Errorf("authentication failed")
		}
		return fmt.Errorf("SOAP error (HTTP %d)", resp.StatusCode)
	}

	return xml.Unmarshal(data, result)
}
