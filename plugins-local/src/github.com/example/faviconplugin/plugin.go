package faviconplugin

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type Rule struct {
	Regex       string `json:"regex,omitempty"`
	FilePath    string `json:"filePath,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

type Config struct {
	Rules []Rule `json:"rules,omitempty"`
}

func CreateConfig() *Config {
	return &Config{}
}

type compiledRule struct {
	re          *regexp.Regexp
	body        []byte
	contentType string
}

type StaticFile struct {
	next  http.Handler
	rules []compiledRule
}

func New(_ context.Context, next http.Handler, config *Config, _ string) (http.Handler, error) {
	if len(config.Rules) == 0 {
		return nil, errors.New("at least one rule is required")
	}

	rules := make([]compiledRule, 0, len(config.Rules))
	for i, r := range config.Rules {
		if r.Regex == "" || r.FilePath == "" {
			return nil, fmt.Errorf("rules[%d]: regex and filePath are required", i)
		}

		re, err := regexp.Compile(r.Regex)
		if err != nil {
			return nil, fmt.Errorf("rules[%d]: invalid regex: %w", i, err)
		}

		body, err := os.ReadFile(r.FilePath)
		if err != nil {
			return nil, fmt.Errorf("rules[%d]: failed to read %s: %w", i, r.FilePath, err)
		}

		contentType := r.ContentType
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(r.FilePath))
		}
		if contentType == "" {
			contentType = http.DetectContentType(body)
		}

		rules = append(rules, compiledRule{re: re, body: body, contentType: contentType})
	}

	return &StaticFile{next: next, rules: rules}, nil
}

func (s *StaticFile) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet || req.Method == http.MethodHead {
		url := requestURL(req)
		for _, r := range s.rules {
			if !r.re.MatchString(url) {
				continue
			}

			rw.Header().Set("Content-Type", r.contentType)
			rw.Header().Set("Content-Length", strconv.Itoa(len(r.body)))
			rw.Header().Set("Cache-Control", "public, max-age=86400")
			rw.WriteHeader(http.StatusOK)

			if req.Method == http.MethodGet {
				_, _ = rw.Write(r.body)
			}
			return
		}
	}

	s.next.ServeHTTP(rw, req)
}

func requestURL(req *http.Request) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + req.Host + req.URL.RequestURI()
}
