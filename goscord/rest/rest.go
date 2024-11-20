package rest

import (
	"errors"
	"fmt"
	"github.com/Goscord/goscord/goscord/rest/ratelimit"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	token string
	rl    *ratelimit.RateLimiter
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		rl:    ratelimit.NewRateLimiter(),
	}
}

func (c *Client) Request(endpoint, method string, data io.Reader, contentType string) ([]byte, error) {
	var req *http.Request

	method = strings.ToUpper(method)
	url := BaseUrl + endpoint
	req, err := http.NewRequest(method, url, data)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "DiscordBot (https://github.com/Goscord/goscord, 1.0.0)")
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", c.token))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	var body []byte

	body, err = io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	resData := make(map[string]interface{})
	_ = sonic.Unmarshal(body, &resData)

	if resp.StatusCode != http.StatusTooManyRequests {
		if msg, ok := resData["message"]; ok {
			return nil, errors.New(msg.(string))
		}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		rl, err := ratelimit.NewRateLimit(resp, body)

		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		if c.rl.Get(rl.Bucket) == nil {
			c.rl.Set(rl.Bucket, rl)
		}

		rl = c.rl.Get(rl.Bucket)
		rl.Wait()

		body, err = c.Request(endpoint, method, data, contentType)
	}

	return body, err
}
