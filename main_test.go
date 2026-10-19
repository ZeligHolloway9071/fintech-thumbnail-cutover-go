package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestThumbnailDeletesUpload(t *testing.T) {
	for _, processOK := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "process failure"}[processOK], func(t *testing.T) {
			deleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/image/upload":
					w.Write([]byte(`{"ok":true,"data":{"image_id":"uploaded-123"}}`))
				case "/v1/image/process":
					if processOK {
						w.Write([]byte(`{"ok":true,"data":{"image":"result"}}`))
					} else {
						w.Write([]byte(`{"ok":false,"error":{"code":"BAD_REQUEST","message":"invalid process"}}`))
					}
				case "/v1/image/delete/uploaded-123":
					if r.Method != http.MethodDelete {
						t.Errorf("delete method: %s", r.Method)
					}
					deleted = true
					w.Write([]byte(`{"ok":true}`))
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
			}))
			defer server.Close()
			c := &client{base: server.URL, key: "test", http: server.Client()}
			_, err := c.thumbnail([]byte("image"), "card.png", 640, 360)
			if (err == nil) != processOK {
				t.Fatalf("processOK=%v, error=%v", processOK, err)
			}
			if !deleted {
				t.Fatal("uploaded image was not deleted")
			}
		})
	}
}

func TestThumbnailDimensions(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		want          bool
	}{
		{"mobile portrait", 360, 640, true},
		{"card landscape", 640, 360, true},
		{"oversized", 0, 360, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.width > 0 && tt.height > 0 && tt.width <= 1600 && tt.height <= 1600
			if got != tt.want {
				t.Fatalf("dimensions %dx%d: got %v, want %v", tt.width, tt.height, got, tt.want)
			}
		})
	}
}
