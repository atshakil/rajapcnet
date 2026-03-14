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
	base  string
	http  *http.Client
	token string
}

func New(baseURL string) *Client {
	return &Client{
		base: baseURL,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) SetToken(token string) {
	c.token = token
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

// ── Users ──

func (c *Client) ListUsers() ([]model.User, error) {
	resp, err := c.get("/api/users")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var users []model.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) GetUser(id string) (*model.User, error) {
	resp, err := c.get("/api/users/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var u model.User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) AddUser(u *model.User) (*model.User, error) {
	resp, err := c.postJSON("/api/users", u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result model.User
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Bootstrap(u *model.User) (*model.User, error) {
	resp, err := c.postJSON("/api/bootstrap", u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result model.User
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Login(username, password string) (string, error) {
	resp, err := c.postJSON("/api/login", map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}

func (c *Client) UpdateUser(id string, u *model.User) error {
	resp, err := c.putJSON("/api/users/"+id, u)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) DeleteUser(id string) error {
	resp, err := c.do("DELETE", "/api/users/"+id, nil)
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
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
