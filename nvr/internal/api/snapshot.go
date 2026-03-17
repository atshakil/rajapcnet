package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func (h *handler) cameraSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var username, password, snapshotURIs string
	err = h.db.QueryRow(
		`SELECT username, password, snapshot_uris FROM cameras WHERE id = ? AND enabled = 1`, id,
	).Scan(&username, &password, &snapshotURIs)
	if err != nil {
		http.Error(w, "camera not found", http.StatusNotFound)
		return
	}

	var uris []string
	if err := json.Unmarshal([]byte(snapshotURIs), &uris); err != nil || len(uris) == 0 {
		http.Error(w, "no snapshot URI configured", http.StatusNotFound)
		return
	}

	data, contentType, err := httpGetDigest(uris[0], username, password)
	if err != nil {
		http.Error(w, "snapshot fetch failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}

// httpGetDigest performs an HTTP GET with Digest authentication.
// It sends an unauthenticated request first; on 401 it parses the
// WWW-Authenticate header and retries with the Digest credentials.
func httpGetDigest(rawURL, username, password string) ([]byte, string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // handle manually if needed
		},
	}

	// First request — unauthenticated
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("first GET: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Camera allows unauthenticated snapshots — retry to read body
		req2, _ := http.NewRequest("GET", rawURL, nil)
		resp2, err := client.Do(req2)
		if err != nil {
			return nil, "", fmt.Errorf("GET: %w", err)
		}
		defer resp2.Body.Close()
		data, err := io.ReadAll(resp2.Body)
		return data, resp2.Header.Get("Content-Type"), err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Parse WWW-Authenticate: Digest realm="...", nonce="...", ...
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	realm := digestParam(wwwAuth, "realm")
	nonce := digestParam(wwwAuth, "nonce")
	qop := digestParam(wwwAuth, "qop")

	nc := "00000001"
	cnonce := fmt.Sprintf("%08x", rand.Uint32())
	uri := rawURL
	// Use only the path+query portion for the uri field
	if len(rawURL) > 7 {
		if idx := strings.Index(rawURL[8:], "/"); idx >= 0 {
			rest := rawURL[8:]
			if si := strings.Index(rest, "/"); si >= 0 {
				uri = rest[si:]
			}
		}
	}

	ha1 := md5hex(username + ":" + realm + ":" + password)
	ha2 := md5hex("GET:" + uri)

	var response string
	if strings.Contains(qop, "auth") {
		response = md5hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
	} else {
		response = md5hex(ha1 + ":" + nonce + ":" + ha2)
	}

	var authHeader string
	if strings.Contains(qop, "auth") {
		authHeader = fmt.Sprintf(
			`Digest username="%s", realm="%s", nonce="%s", uri="%s", qop=%s, nc=%s, cnonce="%s", response="%s"`,
			username, realm, nonce, uri, qop, nc, cnonce, response,
		)
	} else {
		authHeader = fmt.Sprintf(
			`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
			username, realm, nonce, uri, response,
		)
	}

	req2, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build auth request: %w", err)
	}
	req2.Header.Set("Authorization", authHeader)

	resp2, err := client.Do(req2)
	if err != nil {
		return nil, "", fmt.Errorf("auth GET: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("auth GET status %d", resp2.StatusCode)
	}

	data, err := io.ReadAll(resp2.Body)
	return data, resp2.Header.Get("Content-Type"), err
}

var digestParamRe = regexp.MustCompile(`(?i)(\w+)="([^"]*)"`)

func digestParam(header, key string) string {
	for _, m := range digestParamRe.FindAllStringSubmatch(header, -1) {
		if strings.EqualFold(m[1], key) {
			return m[2]
		}
	}
	return ""
}

func md5hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
