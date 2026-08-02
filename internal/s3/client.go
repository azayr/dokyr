package s3

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type Config struct {
	Region         string
	Bucket         string
	Endpoint       string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
	Secure         bool
}

type Client struct {
	config Config
	http   *http.Client
}

func New(config Config) (*Client, error) {
	config.Region = strings.TrimSpace(config.Region)
	config.Bucket = strings.TrimSpace(config.Bucket)
	config.Endpoint = strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	// R2 documents `auto` as its S3 signing region. Older Dokyr registry
	// settings used the AWS-compatible `us-east-1` alias; normalizing here also
	// covers migrated connections and avoids SignatureDoesNotMatch responses.
	if strings.Contains(strings.ToLower(config.Endpoint), ".r2.cloudflarestorage.com") {
		config.Region = "auto"
	}
	if config.Region == "" || config.Bucket == "" || config.AccessKey == "" || config.SecretKey == "" {
		return nil, errors.New("S3 region, bucket, access key, and secret key are required")
	}
	if config.Endpoint != "" {
		endpoint, err := url.Parse(config.Endpoint)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return nil, errors.New("S3 endpoint is invalid")
		}
	}
	return &Client{config: config, http: &http.Client{Timeout: 30 * time.Minute}}, nil
}

func (c *Client) PutFile(ctx context.Context, key, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	req, err := c.request(ctx, http.MethodPut, key, file, hex.EncodeToString(hash.Sum(nil)), time.Now().UTC())
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/gzip")
	return c.do(req, io.Discard)
}

func (c *Client) GetFile(ctx context.Context, key, filename string) error {
	req, err := c.request(ctx, http.MethodGet, key, nil, emptySHA256, time.Now().UTC())
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := c.do(req, file); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (c *Client) Delete(ctx context.Context, key string) error {
	req, err := c.request(ctx, http.MethodDelete, key, nil, emptySHA256, time.Now().UTC())
	if err != nil {
		return err
	}
	return c.do(req, io.Discard)
}

const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func (c *Client) request(ctx context.Context, method, key string, body io.Reader, payloadHash string, now time.Time) (*http.Request, error) {
	target, err := c.objectURL(key)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	request.Header.Set("x-amz-content-sha256", payloadHash)
	request.Header.Set("x-amz-date", amzDate)

	canonicalHeaders := "host:" + target.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := method + "\n" + target.EscapedPath() + "\n\n" +
		canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	scope := shortDate + "/" + c.config.Region + "/s3/aws4_request"
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(requestHash[:])
	signingKey := signatureKey(c.config.SecretKey, shortDate, c.config.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.config.AccessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)
	return request, nil
}

func (c *Client) objectURL(key string) (*url.URL, error) {
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")
	if key == "" {
		return nil, errors.New("S3 object key is required")
	}
	var target *url.URL
	var err error
	if c.config.Endpoint != "" {
		target, err = url.Parse(c.config.Endpoint)
	} else {
		scheme := "https"
		if !c.config.Secure {
			scheme = "http"
		}
		target, err = url.Parse(scheme + "://s3." + c.config.Region + ".amazonaws.com")
	}
	if err != nil {
		return nil, err
	}
	if c.config.ForcePathStyle {
		target.Path = "/" + path.Join(c.config.Bucket, key)
	} else {
		target.Host = c.config.Bucket + "." + target.Host
		target.Path = "/" + path.Clean(key)
	}
	return target, nil
}

func (c *Client) do(request *http.Request, destination io.Writer) error {
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		detail := strings.TrimSpace(string(message))
		var providerError struct {
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		}
		if xml.Unmarshal(message, &providerError) == nil && strings.TrimSpace(providerError.Message) != "" {
			detail = strings.TrimSpace(providerError.Message)
			if code := strings.TrimSpace(providerError.Code); code != "" {
				detail = code + ": " + detail
			}
		}
		if detail == "" {
			detail = response.Status
		}
		return fmt.Errorf("S3 request failed: %s", detail)
	}
	_, err = io.Copy(destination, response.Body)
	return err
}

func signatureKey(secret, date, region, service string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secret), date)
	regionKey := hmacSHA256(dateKey, region)
	serviceKey := hmacSHA256(regionKey, service)
	return hmacSHA256(serviceKey, "aws4_request")
}

func hmacSHA256(key []byte, value string) []byte {
	hash := hmac.New(sha256.New, key)
	_, _ = hash.Write([]byte(value))
	return hash.Sum(nil)
}
