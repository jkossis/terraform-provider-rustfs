// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	sigV4Algorithm = "AWS4-HMAC-SHA256"
	sigV4DateTime  = "20060102T150405Z"
	sigV4Date      = "20060102"
	sigV4Region    = ""
	sigV4Service   = "s3"
)

var sigV4IgnoredHeaders = map[string]bool{
	"Accept-Encoding": true,
	"Authorization":   true,
	"User-Agent":      true,
}

func signV4(req *http.Request, accessKey, secretKey, sessionToken string) *http.Request {
	if accessKey == "" || secretKey == "" {
		return req
	}

	now := time.Now().UTC()
	req.Header.Set("X-Amz-Date", now.Format(sigV4DateTime))
	if sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", sessionToken)
	}

	hashedPayload := req.Header.Get("X-Amz-Content-Sha256")
	canonicalRequest := canonicalRequest(req, hashedPayload)
	scope := credentialScope(now)
	stringToSign := strings.Join([]string{
		sigV4Algorithm,
		now.Format(sigV4DateTime),
		scope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	signature := hex.EncodeToString(hmacSHA256(signingKey(secretKey, now), []byte(stringToSign)))
	authorization := strings.Join([]string{
		sigV4Algorithm + " Credential=" + accessKey + "/" + scope,
		"SignedHeaders=" + signedHeaders(req),
		"Signature=" + signature,
	}, ", ")
	req.Header.Set("Authorization", authorization)

	return req
}

func credentialScope(t time.Time) string {
	return strings.Join([]string{t.Format(sigV4Date), sigV4Region, sigV4Service, "aws4_request"}, "/")
}

func signingKey(secretKey string, t time.Time) []byte {
	date := hmacSHA256([]byte("AWS4"+secretKey), []byte(t.Format(sigV4Date)))
	region := hmacSHA256(date, []byte(sigV4Region))
	service := hmacSHA256(region, []byte(sigV4Service))
	return hmacSHA256(service, []byte("aws4_request"))
}

func canonicalRequest(req *http.Request, hashedPayload string) string {
	return strings.Join([]string{
		req.Method,
		canonicalURI(req),
		canonicalQuery(req),
		canonicalHeaders(req),
		signedHeaders(req),
		hashedPayload,
	}, "\n")
}

func canonicalURI(req *http.Request) string {
	path := req.URL.EscapedPath()
	if path == "" {
		return "/"
	}

	return path
}

func canonicalQuery(req *http.Request) string {
	return strings.ReplaceAll(req.URL.Query().Encode(), "+", "%20")
}

func canonicalHeaders(req *http.Request) string {
	headers := signedHeaderNames(req)
	values := lowerHeaderValues(req)
	var out strings.Builder

	for _, header := range headers {
		out.WriteString(header)
		out.WriteByte(':')
		if header == "host" {
			out.WriteString(req.Host)
		} else {
			out.WriteString(strings.Join(values[header], ","))
		}
		out.WriteByte('\n')
	}

	return out.String()
}

func signedHeaders(req *http.Request) string {
	return strings.Join(signedHeaderNames(req), ";")
}

func signedHeaderNames(req *http.Request) []string {
	headers := make([]string, 0, len(req.Header)+1)
	hasHost := false

	for name := range req.Header {
		canonicalName := http.CanonicalHeaderKey(name)
		if sigV4IgnoredHeaders[canonicalName] {
			continue
		}
		lowerName := strings.ToLower(name)
		if lowerName == "host" {
			hasHost = true
		}
		headers = append(headers, lowerName)
	}

	if !hasHost {
		headers = append(headers, "host")
	}

	sort.Strings(headers)
	return headers
}

func lowerHeaderValues(req *http.Request) map[string][]string {
	values := make(map[string][]string, len(req.Header))
	for name, headerValues := range req.Header {
		lowerName := strings.ToLower(name)
		values[lowerName] = make([]string, 0, len(headerValues))
		for _, value := range headerValues {
			values[lowerName] = append(values[lowerName], trimHeaderValue(value))
		}
	}
	return values
}

func trimHeaderValue(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func hexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
