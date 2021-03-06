package ridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// PayloadVersion when this variable set, Ridge disables auto detection payload version.
var PayloadVersion string

// RequestV1 represents an HTTP request received by an API Gateway proxy integrations. (v1.0)
// https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html
type RequestV1 struct {
	Body                            string              `json:"body"`
	Headers                         map[string]string   `json:"headers"`
	MultiValueHeaders               http.Header         `json:"multiValueHeaders"`
	HTTPMethod                      string              `json:"httpMethod"`
	Path                            string              `json:"path"`
	PathParameters                  map[string]string   `json:"pathParameters"`
	QueryStringParameters           map[string]string   `json:"queryStringParameters"`
	MultiValueQueryStringParameters map[string][]string `json:"multiValueQueryStringParameters"`
	Resource                        string              `json:"resource"`
	StageVariables                  map[string]string   `json:"stageVariables"`
	RequestContext                  RequestContextV1    `json:"requestContext"`
	IsBase64Encoded                 bool                `json:"isBase64Encoded"`
}

// Request is alias to RequestV1
type Request = RequestV1

// NewRequest creates *net/http.Request from a Request.
func NewRequest(event json.RawMessage) (*http.Request, error) {
	var r struct {
		Version string `json:"version"`
	}
	if PayloadVersion == "" {
		if err := json.Unmarshal(event, &r); err != nil {
			return nil, err
		}
	} else {
		r.Version = PayloadVersion
	}

	switch r.Version {
	case "2.0":
		var rv2 RequestV2
		if err := json.Unmarshal(event, &rv2); err != nil {
			return nil, err
		}
		return rv2.httpRequest()
	case "1.0", "":
		var rv1 RequestV1
		if err := json.Unmarshal(event, &rv1); err != nil {
			return nil, err
		}
		return rv1.httpRequest()
	default:
		return nil, fmt.Errorf("Payload Version %s is not supported", r.Version)
	}
}

func (r RequestV1) httpRequest() (*http.Request, error) {
	header := make(http.Header)
	if len(r.MultiValueHeaders) > 0 {
		for key, values := range r.MultiValueHeaders {
			for _, value := range values {
				header.Add(key, value)
			}
		}
	} else {
		for key, value := range r.Headers {
			header.Add(key, value)
		}
	}
	host := header.Get("Host")
	header.Del("Host")
	v := make(url.Values)
	if len(r.MultiValueQueryStringParameters) > 0 {
		for key, values := range r.MultiValueQueryStringParameters {
			for _, value := range values {
				v.Add(key, value)
			}
		}
	} else {
		for key, value := range r.QueryStringParameters {
			v.Add(key, value)
		}
	}
	uri := r.Path
	if len(r.QueryStringParameters) > 0 {
		uri = uri + "?" + v.Encode()
	}
	u, _ := url.Parse(uri)
	var contentLength int64
	var b io.Reader
	if r.IsBase64Encoded {
		raw := make([]byte, len(r.Body))
		n, err := base64.StdEncoding.Decode(raw, []byte(r.Body))
		if err != nil {
			return nil, err
		}
		contentLength = int64(n)
		b = bytes.NewReader(raw[0:n])
	} else {
		contentLength = int64(len(r.Body))
		b = strings.NewReader(r.Body)
	}
	req := http.Request{
		Method:        r.HTTPMethod,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		ContentLength: contentLength,
		Body:          ioutil.NopCloser(b),
		RemoteAddr:    r.RequestContext.Identity["sourceIp"],
		Host:          host,
		RequestURI:    uri,
		URL:           u,
	}
	return &req, nil
}

// RequestContextV1 represents request contest object (v1.0).
type RequestContextV1 struct {
	AccountID    string            `json:"accountId"`
	APIID        string            `json:"apiId"`
	HTTPMethod   string            `json:"httpMethod"`
	Identity     map[string]string `json:"identity"`
	RequestID    string            `json:"requestId"`
	ResourceID   string            `json:"resourceId"`
	ResourcePath string            `json:"resourcePath"`
	Stage        string            `json:"stage"`
}

// RequstContext is alias to RequestContextV1
type RequetContext = RequestContextV1

// RequestV2 represents an HTTP request received by an API Gateway proxy integrations. (v2.0)
// https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html
type RequestV2 struct {
	Version               string            `json:"version"`
	RouteKey              string            `json:"routeKey"`
	RawPath               string            `json:"rawPath"`
	RawQueryString        string            `json:"rawQueryString"`
	Cookies               []string          `json:"cookies"`
	Headers               map[string]string `json:"headers"`
	QueryStringParameters map[string]string `json:"queryStringParameters"`
	RequestContext        RequestContextV2  `json:"requestContext"`
	Body                  string            `json:"body"`
	IsBase64Encoded       bool              `json:"isBase64Encoded"`
	StageVariables        map[string]string `json:"stageVariables"`
}

// RequestContextV2 represents request context for v2.0
type RequestContextV2 struct {
	AccountID    string `json:"accountId"`
	APIID        string `json:"apiId"`
	DomainName   string `json:"domainName"`
	DomainPrefix string `json:"domainPrefix"`
	HTTP         struct {
		Method    string `json:"method"`
		Path      string `json:"path"`
		Protocol  string `json:"protocol"`
		SourceIP  string `json:"sourceIp"`
		UserAgent string `json:"userAgent"`
	} `json:"http"`
	RequestID string `json:"requestId"`
	RouteID   string `json:"routeId"`
	RouteKey  string `json:"routeKey"`
	Stage     string `json:"stage"`
	Time      string `json:"time"`
	TimeEpoch int64  `json:"timeEpoch"`
}

func (r RequestV2) httpRequest() (*http.Request, error) {
	header := make(http.Header)
	for key, value := range r.Headers {
		header.Add(key, value)
	}
	host := header.Get("Host")
	header.Del("Host")
	uri := r.RawPath
	if r.RawQueryString != "" {
		uri = uri + "?" + r.RawQueryString
	}
	u, _ := url.Parse(uri)
	var contentLength int64
	var b io.Reader
	if r.IsBase64Encoded {
		raw := make([]byte, len(r.Body))
		n, err := base64.StdEncoding.Decode(raw, []byte(r.Body))
		if err != nil {
			return nil, err
		}
		contentLength = int64(n)
		b = bytes.NewReader(raw[0:n])
	} else {
		contentLength = int64(len(r.Body))
		b = strings.NewReader(r.Body)
	}
	req := http.Request{
		Method:        r.RequestContext.HTTP.Method,
		Proto:         r.RequestContext.HTTP.Protocol,
		Header:        header,
		ContentLength: contentLength,
		Body:          ioutil.NopCloser(b),
		RemoteAddr:    r.RequestContext.HTTP.SourceIP,
		Host:          host,
		RequestURI:    uri,
		URL:           u,
	}
	return &req, nil
}
