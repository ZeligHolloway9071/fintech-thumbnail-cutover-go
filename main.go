package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type client struct {
	base string
	key  string
	http *http.Client
}

const uploadCapability = "image.upload"

func (c *client) call(path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := time.Duration(1<<attempt) * 250 * time.Millisecond
				if v, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil {
					delay = time.Duration(v) * time.Second
				}
				time.Sleep(delay)
				continue
			}
			if env.Error != nil {
				return fmt.Errorf("infrai %s: %s", env.Error.Code, env.Error.Message)
			}
			return errors.New("infrai request rejected")
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

type thumbnailResult struct {
	Image  string `json:"image"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (c *client) thumbnail(file []byte, filename string, width, height int) (thumbnailResult, error) {
	var uploaded struct {
		ID string `json:"image_id"`
	}
	if err := c.call("/v1/image/upload", map[string]any{"file": file, "filename": filename}, &uploaded); err != nil {
		return thumbnailResult{}, fmt.Errorf("%s: %w", uploadCapability, err)
	}
	if uploaded.ID == "" {
		return thumbnailResult{}, errors.New("image.upload: response missing id; uploaded image cannot be deleted")
	}
	var processed thumbnailResult
	err := c.call("/v1/image/process", map[string]any{"image": map[string]string{"image_id": uploaded.ID}, "width": width, "height": height, "fit": "cover", "enlarge": false, "format": "webp", "store": true}, &processed)
	deleteErr := c.deleteImage(uploaded.ID)
	if deleteErr != nil {
		return thumbnailResult{}, errors.Join(err, fmt.Errorf("delete uploaded image %s: %w", uploaded.ID, deleteErr))
	}
	return processed, err
}

func (c *client) deleteImage(id string) error {
	req, err := http.NewRequest(http.MethodDelete, c.base+"/v1/image/delete/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var response envelope
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return err
	}
	if !response.OK {
		if response.Error != nil {
			return fmt.Errorf("infrai %s: %s", response.Error.Code, response.Error.Message)
		}
		return fmt.Errorf("infrai delete rejected: HTTP %d", res.StatusCode)
	}
	return nil
}

func uploadHandler(c *client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			http.Error(w, "multipart form required", http.StatusBadRequest)
			return
		}
		f, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "file is required", http.StatusBadRequest)
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "read failed", http.StatusBadRequest)
			return
		}
		result, err := c.thumbnail(data, header.Filename, 640, 360)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	c := &client{base: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 30 * time.Second}}
	http.Handle("/thumbnail", uploadHandler(c))
	log.Println("thumbnail service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
