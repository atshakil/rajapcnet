package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"nvr/internal/model"
)

type Client struct {
	base   string
	http   *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		base: baseURL,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Health() (string, error) {
	resp, err := c.get("/api/health")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result["status"], nil
}

func (c *Client) ListCameras() ([]model.Camera, error) {
	resp, err := c.get("/api/cameras")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var cameras []model.Camera
	if err := json.NewDecoder(resp.Body).Decode(&cameras); err != nil {
		return nil, err
	}
	return cameras, nil
}

func (c *Client) GetCamera(id string) (*model.Camera, error) {
	resp, err := c.get("/api/cameras/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var cam model.Camera
	if err := json.NewDecoder(resp.Body).Decode(&cam); err != nil {
		return nil, err
	}
	return &cam, nil
}

func (c *Client) AddCamera(cam *model.Camera) (*model.Camera, error) {
	resp, err := c.postJSON("/api/cameras", cam)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result model.Camera
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateCamera(id string, cam *model.Camera) error {
	resp, err := c.putJSON("/api/cameras/"+id, cam)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) DeleteCamera(id string) error {
	resp, err := c.do("DELETE", "/api/cameras/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) get(path string) (*http.Response, error) {
	return c.do("GET", path, nil)
}

func (c *Client) postJSON(path string, v any) (*http.Response, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return c.do("POST", path, bytes.NewReader(body))
}

func (c *Client) putJSON(path string, v any) (*http.Response, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return c.do("PUT", path, bytes.NewReader(body))
}

func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server: %s %s → %d: %s", method, path, resp.StatusCode, bytes.TrimSpace(msg))
	}
	return resp, nil
}
