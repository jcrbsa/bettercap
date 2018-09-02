package modules

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type JSRequest struct {
	Client      string
	Method      string
	Version     string
	Scheme      string
	Path        string
	Query       string
	Hostname    string
	ContentType string
	Headers     string
	Body        string

	req      *http.Request
	refHash  string
	bodyRead bool
}

var header_regexp = regexp.MustCompile(`(.*?): (.*)`)

func NewJSRequest(req *http.Request) *JSRequest {
	headers := ""
	cType := ""

	for name, values := range req.Header {
		for _, value := range values {
			headers += name + ": " + value + "\r\n"

			if strings.ToLower(name) == "content-type" {
				cType = value
			}
		}
	}

	jreq := &JSRequest{
		Client:      strings.Split(req.RemoteAddr, ":")[0],
		Method:      req.Method,
		Version:     fmt.Sprintf("%d.%d", req.ProtoMajor, req.ProtoMinor),
		Scheme:      req.URL.Scheme,
		Hostname:    req.Host,
		Path:        req.URL.Path,
		Query:       req.URL.RawQuery,
		ContentType: cType,
		Headers:     headers,

		req:      req,
		bodyRead: false,
	}
	jreq.UpdateHash()

	return jreq
}

func (j *JSRequest) NewHash() string {
	hash := fmt.Sprintf("%s.%s.%s.%s.%s.%s.%s.%s.%s", j.Client, j.Method, j.Version, j.Scheme, j.Hostname, j.Path, j.Query, j.ContentType, j.Headers)
	hash += "." + j.Body
	return hash
}

func (j *JSRequest) UpdateHash() {
	j.refHash = j.NewHash()
}

func (j *JSRequest) WasModified() bool {
	// body was read
	if j.bodyRead {
		return true
	}
	// check if any of the fields has been changed
	return j.NewHash() != j.refHash
}

func (j *JSRequest) GetHeader(name, deflt string) string {
	headers := strings.Split(j.Headers, "\r\n")
	for i := 0; i < len(headers); i++ {
		header_name := header_regexp.ReplaceAllString(headers[i], "$1")
		header_value := header_regexp.ReplaceAllString(headers[i], "$2")

		if strings.ToLower(name) == strings.ToLower(header_name) {
			return header_value
		}
	}
	return deflt
}

func (j *JSRequest) SetHeader(name, value string) {
	headers := strings.Split(j.Headers, "\r\n")
	for i := 0; i < len(headers); i++ {
		header_name := header_regexp.ReplaceAllString(headers[i], "$1")
		header_value := header_regexp.ReplaceAllString(headers[i], "$2")

		if strings.ToLower(name) == strings.ToLower(header_name) {
			old_header := header_name + ": " + header_value + "\r\n"
			new_header := header_name + ": " + value + "\r\n"
			j.Headers = strings.Replace(j.Headers, old_header, new_header, 1)
			return
		}
	}
	j.Headers += name + ": " + value + "\r\n"
}

func (j *JSRequest) RemoveHeader(name string) {
	headers := strings.Split(j.Headers, "\r\n")
	for i := 0; i < len(headers); i++ {
		header_name := header_regexp.ReplaceAllString(headers[i], "$1")
		header_value := header_regexp.ReplaceAllString(headers[i], "$2")

		if strings.ToLower(name) == strings.ToLower(header_name) {
			removed_header := header_name + ": " + header_value + "\r\n"
			j.Headers = strings.Replace(j.Headers, removed_header, "", 1)
			return
		}
	}
}

func (j *JSRequest) ReadBody() string {
	raw, err := ioutil.ReadAll(j.req.Body)
	if err != nil {
		return ""
	}

	j.Body = string(raw)
	j.bodyRead = true
	// reset the request body to the original unread state
	j.req.Body = ioutil.NopCloser(bytes.NewBuffer(raw))

	return j.Body
}

func (j *JSRequest) ParseForm() map[string]string {
	if j.Body == "" {
		j.Body = j.ReadBody()
	}

	form := make(map[string]string)
	parts := strings.Split(j.Body, "&")

	for _, part := range parts {
		nv := strings.SplitN(part, "=", 2)
		if len(nv) == 2 {
			unescaped, err := url.QueryUnescape(nv[1])
			if err == nil {
				form[nv[0]] = unescaped
			} else {
				form[nv[0]] = nv[1]
			}
		}
	}

	return form
}

func (j *JSRequest) ToRequest() (req *http.Request) {
	url := fmt.Sprintf("%s://%s:%s%s?%s", j.Scheme, j.Hostname, j.req.URL.Port(), j.Path, j.Query)
	if j.Body == "" {
		req, _ = http.NewRequest(j.Method, url, j.req.Body)
	} else {
		req, _ = http.NewRequest(j.Method, url, strings.NewReader(j.Body))
	}

	hadType := false

	headers := strings.Split(j.Headers, "\r\n")
	for i := 0; i < len(headers); i++ {
		if headers[i] != "" {
			header_name := header_regexp.ReplaceAllString(headers[i], "$1")
			header_value := header_regexp.ReplaceAllString(headers[i], "$2")

			req.Header.Set(header_name, header_value)
			if strings.ToLower(header_name) == "content-type" {
				hadType = true
			}
		}
	}

	if !hadType && j.ContentType != "" {
		req.Header.Set("Content-Type", j.ContentType)
	}

	return
}
