package http

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	apiUrl       string
	interval     time.Duration
	resultFormat string
	lastHash     string
}

func NewHttpClient(apiUrl string, resultFormat string, interval time.Duration) (*Client, error) {
	log.Printf("[Info] Creating new HTTP client for URL: %s with interval: %s", apiUrl, interval)
	return &Client{
		apiUrl:       apiUrl,
		interval:     interval,
		resultFormat: resultFormat,
	}, nil
}

func (c *Client) sendGetRequest() ([]byte, error) {
	resp, err := http.Get(c.apiUrl)
	if err != nil {
		log.Printf("[Error] Failed to send GET request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Warning] Received response status code: %s.", resp.Status)
		return nil, fmt.Errorf("fail to get config: %v", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Error] Failed to read response body: %v", err)
		return nil, err
	}

	if len(data) == 0 {
		log.Println("[Warning] Response body is empty.")
		return nil, nil
	}

	return data, nil
}

func (c *Client) GetValues(keys []string) (map[string]string, error) {
	log.Printf("[Info] Getting value from %s", c.apiUrl)
	data, err := c.sendGetRequest()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)

	if c.resultFormat == "json" {
		var jsonData map[string]interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			return nil, err
		}

		for k, v := range jsonData {
			result[k] = fmt.Sprint(v)
		}
	}
	if c.resultFormat == "raw" {
		result["raw"] = string(data)
	}
	return result, nil
}

func (c *Client) WatchPrefix(prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	log.Printf("[Info] Watching %s every %s", c.apiUrl, c.interval.String())
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			data, err := c.sendGetRequest()
			if err != nil {
				return waitIndex, err
			}

			hasher := md5.New()
			hasher.Write(data)
			newHash := hex.EncodeToString(hasher.Sum(nil))

			if newHash != c.lastHash {
				c.lastHash = newHash
				log.Printf("[Info] Content hash changed to %s", newHash)
				return uint64(time.Now().UnixNano()), nil // Returns a new, 'larger' index to trigger an update.
			}
		case <-stopChan:
			log.Printf("[Info] Stop watching %s", c.apiUrl)
			return waitIndex, nil
		}
	}
}
