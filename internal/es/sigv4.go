package es

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// sigv4Signer is a hand port of aws4fetch (github.com/mhart/aws4fetch,
// MIT © Michael Hart), pinned to the header-signing mode elasticvue uses:
// service from the connection, signQuery=false, allHeaders=false.
type sigv4Signer struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	region          string
	service         string

	// now is swappable for deterministic tests.
	now func() time.Time
}

func newSigV4Signer(accessKeyID, secretAccessKey, sessionToken, region, service string) (*sigv4Signer, error) {
	if accessKeyID == "" {
		return nil, fmt.Errorf("sigv4: accessKeyId is required")
	}
	if secretAccessKey == "" {
		return nil, fmt.Errorf("sigv4: secretAccessKey is required")
	}
	if region == "" {
		region = "us-east-1"
	}
	return &sigv4Signer{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		sessionToken:    sessionToken,
		region:          region,
		service:         service,
		now:             time.Now,
	}, nil
}

// unsignableHeaders mirrors aws4fetch's UNSIGNABLE_HEADERS (from aws-sdk-js).
var unsignableHeaders = map[string]bool{
	"authorization":     true,
	"content-type":      true,
	"content-length":    true,
	"user-agent":        true,
	"presigned-expires": true,
	"expect":            true,
	"x-amzn-trace-id":   true,
	"range":             true,
	"connection":        true,
}

// sign adds X-Amz-Date (and X-Amz-Security-Token when present) plus the
// Authorization header to req, exactly as AwsV4Signer.sign does.
func (s *sigv4Signer) sign(_ context.Context, req *http.Request) error {
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		body = b
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}

	datetime := s.now().UTC().Format("20060102T150405Z")
	date := datetime[:8]

	headers := map[string]string{}
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		if lk == "host" {
			continue // aws4fetch deletes Host
		}
		headers[lk] = strings.Join(v, ", ")
	}
	headers["x-amz-date"] = datetime
	if s.sessionToken != "" {
		headers["x-amz-security-token"] = s.sessionToken
	}

	host := strings.ToLower(req.URL.Host)

	signable := []string{"host"}
	for k := range headers {
		if !unsignableHeaders[k] {
			signable = append(signable, k)
		}
	}
	sort.Strings(signable)

	var canonHeaders strings.Builder
	for _, h := range signable {
		value := host
		if h != "host" {
			value = collapseWS(headers[h])
		}
		canonHeaders.WriteString(h)
		canonHeaders.WriteByte(':')
		canonHeaders.WriteString(value)
		canonHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(signable, ";")
	credential := date + "/" + s.region + "/" + s.service + "/aws4_request"

	encodedPath := collapseSlashes(req.URL.EscapedPath())
	encodedPath = encodeRfc3986(encodedPath, true)
	encodedSearch := encodedQuery(req.URL.Query())

	bodyHash := sha256Hex(body)

	canonical := strings.Join([]string{
		strings.ToUpper(req.Method),
		encodedPath,
		encodedSearch,
		canonHeaders.String(),
		signedHeaders,
		bodyHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		datetime,
		credential,
		sha256Hex([]byte(canonical)),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+s.secretAccessKey), date)
	kRegion := hmacSHA256(kDate, s.region)
	kService := hmacSHA256(kRegion, s.service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	req.Header.Set("X-Amz-Date", datetime)
	if s.sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", s.sessionToken)
	}
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKeyID, credential, signedHeaders, signature,
	))
	return nil
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// collapseWS mirrors JS .replace(/\s+/g, ' ').
func collapseWS(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		isSpace := r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
		if isSpace {
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}

// collapseSlashes mirrors JS .replace(/\/+/g, '/').
func collapseSlashes(s string) string {
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	return s
}

func isRfc3986Unreserved(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

// encodeRfc3986 percent-encodes everything but unreserved characters (and
// '/' when keepSlash). This equals aws4fetch's encodeURIComponent followed
// by encodeRfc3986, with %2F restored for paths.
func encodeRfc3986(s string, keepSlash bool) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if isRfc3986Unreserved(c) || (keepSlash && c == '/') {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// encodedQuery mirrors aws4fetch's encodedSearch: encodeURIComponent each
// key and value (strict RFC3986), sort by key then value, join with &.
func encodedQuery(q map[string][]string) string {
	type kv struct{ k, v string }
	var pairs []kv
	for k, vs := range q {
		if k == "" {
			continue // no empty keys
		}
		ek := encodeRfc3986(k, false)
		for _, v := range vs {
			pairs = append(pairs, kv{ek, encodeRfc3986(v, false)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	parts := make([]string, len(pairs))
	for i := range pairs {
		parts[i] = pairs[i].k + "=" + pairs[i].v
	}
	return strings.Join(parts, "&")
}
