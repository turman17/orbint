package celestrak

import (
	"fmt"
	"io"
	"net/http"

	"github.com/turman17/orbint/internal/orbit"
)

type Client struct {
	client  *http.Client
	baseURL string
}

func NewClient() *Client {
	return &Client{
		client:  &http.Client{},
		baseURL: "https://celestrak.org/NORAD/elements/gp.php",
	}
}

func (c *Client) fetch(url string) ([]orbit.TLE, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("celestrak returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	return orbit.ParseOMMJSON(body)
}

func (c *Client) FetchGroup(group string) ([]orbit.TLE, error) {
	url := fmt.Sprintf("%s?GROUP=%s&FORMAT=JSON", c.baseURL, group)
	return c.fetch(url)
}

func (c *Client)FetchByCatalogNumber(id int) ([]orbit.TLE, error){
	url := fmt.Sprintf("%s?CATNR=%d&FORMAT=JSON", c.baseURL, id)
	return c.fetch(url)
}