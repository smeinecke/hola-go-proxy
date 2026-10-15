package main

import (
	"encoding/json"
	"os"
)

// Session holds the reusable Hola session credentials. The file format
// is shared with python-hola's ~/.hola_session.json so both clients can
// reuse the same session.
type Session struct {
	UUID       string `json:"uuid"`
	SessionKey int64  `json:"sessionKey"`
}

func loadSession(path string) (*Session, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, nil
	}
	if sess.UUID == "" || sess.SessionKey == 0 {
		return nil, nil
	}
	return &sess, nil
}

func (s *Session) save(path string) error {
	if path == "" || s == nil || s.UUID == "" || s.SessionKey == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s)
}
