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
	Resolutions  []string
	StreamURIs   []string
	SnapshotURIs []string
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

// ---------------------------------------------------------------------------
// H.265 → H.264 codec switching

// getServicesResp is used to discover Media2 service URL via GetServices.
type getServicesResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			Services []struct {
				Namespace string `xml:"Namespace"`
				XAddr     string `xml:"XAddr"`
			} `xml:"Service"`
		} `xml:"GetServicesResponse"`
	} `xml:"Body"`
}

// vec2Cfg holds a ONVIF Media2 VideoEncoder2Configuration.
// Note: Hikvision Media2 returns FrameRateLimit as a float (e.g. "15.000000")
// and ConstantBitRate as an XML attribute (not a child element).
type vec2Cfg struct {
	Token      string `xml:"token,attr"`
	Name       string `xml:"Name"`
	UseCount   int    `xml:"UseCount"`
	Encoding   string `xml:"Encoding"`
	Resolution struct {
		Width  int `xml:"Width"`
		Height int `xml:"Height"`
	} `xml:"Resolution"`
	Quality     float64 `xml:"Quality"`
	RateControl struct {
		ConstantBitRate bool    `xml:"ConstantBitRate,attr"` // attribute in Hikvision response
		FrameRateLimit  float64 `xml:"FrameRateLimit"`       // float in Hikvision Media2
		BitrateLimit    int     `xml:"BitrateLimit"`
	} `xml:"RateControl"`
	Multicast struct {
		Address struct {
			Type        string `xml:"Type"`
			IPv4Address string `xml:"IPv4Address"`
		} `xml:"Address"`
		Port      int  `xml:"Port"`
		TTL       int  `xml:"TTL"`
		AutoStart bool `xml:"AutoStart"`
	} `xml:"Multicast"`
	SessionTimeout string `xml:"SessionTimeout"`
}

type vec2Resp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			Configs []vec2Cfg `xml:"Configurations"`
		} `xml:"GetVideoEncoderConfigurationsResponse"`
	} `xml:"Body"`
}

// videoEncCfg holds a ONVIF VideoEncoderConfiguration as parsed from SOAP XML.
type videoEncCfg struct {
	Token      string `xml:"token,attr"`
	Name       string `xml:"Name"`
	UseCount   int    `xml:"UseCount"`
	Encoding   string `xml:"Encoding"`
	Resolution struct {
		Width  int `xml:"Width"`
		Height int `xml:"Height"`
	} `xml:"Resolution"`
	Quality     float64 `xml:"Quality"`
	RateControl struct {
		FrameRateLimit   int `xml:"FrameRateLimit"`
		EncodingInterval int `xml:"EncodingInterval"`
		BitrateLimit     int `xml:"BitrateLimit"`
	} `xml:"RateControl"`
	H264 *struct {
		GovLength   int    `xml:"GovLength"`
		H264Profile string `xml:"H264Profile"`
	} `xml:"H264"`
	H265 *struct {
		GovLength   int    `xml:"GovLength"`
		H265Profile string `xml:"H265Profile"`
	} `xml:"H265"`
	Multicast struct {
		Address struct {
			Type        string `xml:"Type"`
			IPv4Address string `xml:"IPv4Address"`
		} `xml:"Address"`
		Port      int  `xml:"Port"`
		TTL       int  `xml:"TTL"`
		AutoStart bool `xml:"AutoStart"`
	} `xml:"Multicast"`
	SessionTimeout string `xml:"SessionTimeout"`
}

type vecResp struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Resp struct {
			Configs []videoEncCfg `xml:"Configurations"`
		} `xml:"GetVideoEncoderConfigurationsResponse"`
	} `xml:"Body"`
}

// EnsureH264 checks all video encoder configurations on the camera via ONVIF.
// Any configuration currently set to H.265 is switched to H.264 (Main profile).
// It checks both ONVIF Media 1.x and Media 2.0 (Hikvision exposes H.265 only
// via Media 2.0 while Media 1.x always reports H.264).
// Returns (changed bool, err error).
func EnsureH264(ip string, port int, onvifPath, username, password string) (bool, error) {
	baseURL := fmt.Sprintf("http://%s:%d%s", ip, port, onvifPath)

	// Discover media service URLs (Media1 from GetCapabilities, Media2 from GetServices)
	var caps capabilitiesResp
	soapCallAuth(baseURL,
		cleanEnvelope(`<tds:GetCapabilities xmlns:tds="http://www.onvif.org/ver10/device/wsdl"><tds:Category>Media</tds:Category></tds:GetCapabilities>`),
		"http://www.onvif.org/ver10/device/wsdl/GetCapabilities",
		username, password, &caps) //nolint:errcheck
	mediaURL := baseURL
	if caps.Body.Resp.Capabilities.Media.XAddr != "" {
		mediaURL = caps.Body.Resp.Capabilities.Media.XAddr
	}

	// Discover Media2 URL via GetServices
	media2URL := ""
	var svcs getServicesResp
	if err := soapCallAuth(baseURL,
		cleanEnvelope(`<tds:GetServices xmlns:tds="http://www.onvif.org/ver10/device/wsdl"><tds:IncludeCapability>false</tds:IncludeCapability></tds:GetServices>`),
		"http://www.onvif.org/ver10/device/wsdl/GetServices",
		username, password, &svcs); err == nil {
		for _, svc := range svcs.Body.Resp.Services {
			if strings.Contains(svc.Namespace, "ver20/media") {
				media2URL = svc.XAddr
				break
			}
		}
	}
	// Fallback: try common Media2 path if GetServices didn't help
	if media2URL == "" {
		// Derive from Media1 URL or base
		base := fmt.Sprintf("http://%s:%d", ip, port)
		media2URL = base + "/onvif/Media2"
	}

	isH265 := func(enc string) bool {
		enc = strings.ToUpper(strings.TrimSpace(enc))
		return enc == "H265" || enc == "H.265" || enc == "HEVC"
	}

	changed := false

	// --- Media 1.x pass ---
	var cfgs vecResp
	if err := soapCallAuth(mediaURL,
		cleanEnvelope(`<trt:GetVideoEncoderConfigurations xmlns:trt="http://www.onvif.org/ver10/media/wsdl"/>`),
		"http://www.onvif.org/ver10/media/wsdl/GetVideoEncoderConfigurations",
		username, password, &cfgs); err != nil {
		return false, fmt.Errorf("GetVideoEncoderConfigurations: %w", err)
	}
	for _, c := range cfgs.Body.Resp.Configs {
		if !isH265(c.Encoding) {
			continue
		}
		body := buildSetVECH264Body(c)
		var dummy struct{}
		if err := soapCallAuth(mediaURL, cleanEnvelope(body),
			"http://www.onvif.org/ver10/media/wsdl/SetVideoEncoderConfiguration",
			username, password, &dummy); err != nil {
			return changed, fmt.Errorf("SetVideoEncoderConfiguration token=%s: %w", c.Token, err)
		}
		changed = true
	}

	// --- Media 2.0 pass (Hikvision reports H265 here, not in Media1) ---
	var cfgs2 vec2Resp
	if err := soapCallAuth(media2URL,
		cleanEnvelope(`<tr2:GetVideoEncoderConfigurations xmlns:tr2="http://www.onvif.org/ver20/media/wsdl"/>`),
		"http://www.onvif.org/ver20/media/wsdl/GetVideoEncoderConfigurations",
		username, password, &cfgs2); err == nil {
		for _, c := range cfgs2.Body.Resp.Configs {
			if !isH265(c.Encoding) {
				continue
			}
			body := buildSetVEC2H264Body(c)
			var dummy struct{}
			if err2 := soapCallAuth(media2URL, cleanEnvelope(body),
				"http://www.onvif.org/ver20/media/wsdl/SetVideoEncoderConfiguration",
				username, password, &dummy); err2 != nil {
				return changed, fmt.Errorf("Media2 SetVideoEncoderConfiguration token=%s: %w", c.Token, err2)
			}
			changed = true
		}
	}

	return changed, nil
}

// buildSetVECH264Body constructs the ONVIF SetVideoEncoderConfiguration SOAP body
// that changes encoding to H.264 while preserving all other settings.
func buildSetVECH264Body(c videoEncCfg) string {
	govLen := 50
	if c.H265 != nil && c.H265.GovLength > 0 {
		govLen = c.H265.GovLength
	} else if c.H264 != nil && c.H264.GovLength > 0 {
		govLen = c.H264.GovLength
	}

	quality := c.Quality
	if quality <= 0 {
		quality = 7.5
	}
	fps := c.RateControl.FrameRateLimit
	if fps <= 0 {
		fps = 25
	}
	interval := c.RateControl.EncodingInterval
	if interval <= 0 {
		interval = 1
	}
	kbps := c.RateControl.BitrateLimit
	if kbps <= 0 {
		kbps = 2048
	}
	timeout := c.SessionTimeout
	if timeout == "" {
		timeout = "PT60S"
	}
	ipType := c.Multicast.Address.Type
	if ipType == "" {
		ipType = "IPv4"
	}
	ipv4 := c.Multicast.Address.IPv4Address
	if ipv4 == "" {
		ipv4 = "0.0.0.0"
	}
	autoStart := "false"
	if c.Multicast.AutoStart {
		autoStart = "true"
	}

	return fmt.Sprintf(
		`<trt:SetVideoEncoderConfiguration xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">`+
			`<trt:Configuration token="%s">`+
			`<tt:Name>%s</tt:Name>`+
			`<tt:UseCount>%d</tt:UseCount>`+
			`<tt:Encoding>H264</tt:Encoding>`+
			`<tt:Resolution><tt:Width>%d</tt:Width><tt:Height>%d</tt:Height></tt:Resolution>`+
			`<tt:Quality>%.4f</tt:Quality>`+
			`<tt:RateControl><tt:FrameRateLimit>%d</tt:FrameRateLimit><tt:EncodingInterval>%d</tt:EncodingInterval><tt:BitrateLimit>%d</tt:BitrateLimit></tt:RateControl>`+
			`<tt:H264><tt:GovLength>%d</tt:GovLength><tt:H264Profile>Main</tt:H264Profile></tt:H264>`+
			`<tt:Multicast><tt:Address><tt:Type>%s</tt:Type><tt:IPv4Address>%s</tt:IPv4Address></tt:Address><tt:Port>%d</tt:Port><tt:TTL>%d</tt:TTL><tt:AutoStart>%s</tt:AutoStart></tt:Multicast>`+
			`<tt:SessionTimeout>%s</tt:SessionTimeout>`+
			`</trt:Configuration>`+
			`<trt:ForcePersistence>true</trt:ForcePersistence>`+
			`</trt:SetVideoEncoderConfiguration>`,
		xmlEsc(c.Token), xmlEsc(c.Name), c.UseCount,
		c.Resolution.Width, c.Resolution.Height,
		quality,
		fps, interval, kbps,
		govLen,
		xmlEsc(ipType), xmlEsc(ipv4), c.Multicast.Port, c.Multicast.TTL, autoStart,
		xmlEsc(timeout),
	)
}

// xmlEsc returns s with XML special characters escaped.
func xmlEsc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s)) //nolint:errcheck
	return b.String()
}

// buildSetVEC2H264Body constructs an ONVIF Media 2.0 SetVideoEncoderConfiguration
// SOAP body that switches a vec2Cfg (H.265) to H.264.
// Note: Hikvision Media2 uses attributes for GovLength/Profile/ConstantBitRate
// and does not have H264/H265 child elements.
func buildSetVEC2H264Body(c vec2Cfg) string {
	quality := c.Quality
	if quality <= 0 {
		quality = 7.5
	}
	fps := c.RateControl.FrameRateLimit
	if fps <= 0 {
		fps = 25
	}
	kbps := c.RateControl.BitrateLimit
	if kbps <= 0 {
		kbps = 2048
	}
	timeout := c.SessionTimeout
	if timeout == "" {
		timeout = "PT0S"
	}
	ipType := c.Multicast.Address.Type
	if ipType == "" {
		ipType = "IPv4"
	}
	ipv4 := c.Multicast.Address.IPv4Address
	if ipv4 == "" {
		ipv4 = "0.0.0.0"
	}
	autoStart := "false"
	if c.Multicast.AutoStart {
		autoStart = "true"
	}
	cbr := "false"
	if c.RateControl.ConstantBitRate {
		cbr = "true"
	}

	return fmt.Sprintf(
		`<tr2:SetVideoEncoderConfiguration xmlns:tr2="http://www.onvif.org/ver20/media/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">`+
			`<tr2:Configuration token="%s">`+
			`<tt:Name>%s</tt:Name>`+
			`<tt:UseCount>%d</tt:UseCount>`+
			`<tt:Encoding>H264</tt:Encoding>`+
			`<tt:Resolution><tt:Width>%d</tt:Width><tt:Height>%d</tt:Height></tt:Resolution>`+
			`<tt:RateControl ConstantBitRate="%s"><tt:FrameRateLimit>%g</tt:FrameRateLimit><tt:BitrateLimit>%d</tt:BitrateLimit></tt:RateControl>`+
			`<tt:Multicast><tt:Address><tt:Type>%s</tt:Type><tt:IPv4Address>%s</tt:IPv4Address></tt:Address><tt:Port>%d</tt:Port><tt:TTL>%d</tt:TTL><tt:AutoStart>%s</tt:AutoStart></tt:Multicast>`+
			`<tt:Quality>%.6f</tt:Quality>`+
			`</tr2:Configuration>`+
			`</tr2:SetVideoEncoderConfiguration>`,
		xmlEsc(c.Token), xmlEsc(c.Name), c.UseCount,
		c.Resolution.Width, c.Resolution.Height,
		cbr, fps, kbps,
		xmlEsc(ipType), xmlEsc(ipv4), c.Multicast.Port, c.Multicast.TTL, autoStart,
		quality,
	)
}
