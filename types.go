package main

import "encoding/json"

type PluginArgs struct {
	Key         string            `json:"access_key"`
	Secret      string            `json:"secret_key"`
	Bucket      string            `json:"bucket"`
	Region      string            `json:"region"`
	Source      string            `json:"source"`
	Target      string            `json:"target"`
	Delete      bool              `json:"delete"`
	Access      StringMap         `json:"acl"`
	ContentType StringMap         `json:"content_type"`
	Metadata    DeepStringMap     `json:"metadata"`
	Redirects   map[string]string `json:"redirects"`
}

type DeepStringMap struct {
	parts map[string]map[string]string
}

func (e *DeepStringMap) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	p := map[string]map[string]string{}
	if err := json.Unmarshal(b, &p); err != nil {
		s := map[string]string{}
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		p["*"] = s
	}

	e.parts = p
	return nil
}

func (e *DeepStringMap) Map() map[string]map[string]string {
	return e.parts
}

type StringMap struct {
	parts map[string]string
}

func (e *StringMap) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	p := map[string]string{}
	if err := json.Unmarshal(b, &p); err != nil {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		p["_string_"] = s
	}

	e.parts = p
	return nil
}

func (e *StringMap) IsEmpty() bool {
	if e == nil || len(e.parts) == 0 {
		return true
	}

	return false
}

func (e *StringMap) IsString() bool {
	if e.IsEmpty() || len(e.parts) != 1 {
		return false
	}

	_, ok := e.parts["_string_"]
	return ok
}

func (e *StringMap) String() string {
	if e.IsEmpty() || !e.IsString() {
		return ""
	}

	return e.parts["_string_"]
}

func (e *StringMap) Map() map[string]string {
	if e.IsEmpty() || e.IsString() {
		return map[string]string{}
	}

	return e.parts
}
