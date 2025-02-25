/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sdk

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// DefaultTimeout api requests after 180s
const DefaultTimeout = 180 * time.Second

// Endpoints

var VKE = os.Getenv("VKE_URL")

// Endpoints conveniently maps endpoints names to their URI for external configuration
var Endpoints = map[string]string{
	"vke": VKE,
}

// Errors
var (
	ErrAPIDown = errors.New("The VKE API is down, it does't respond to /time anymore")
)

type Client struct {
	AppKey string

	// AppSecret holds the Application secret key
	AppSecret string

	// API endpoint
	endpoint string

	// Client is the underlying HTTP client used to run the requests. It may be overloaded but a default one is instanciated in ``NewClient`` by default.
	Client *http.Client

	// Logger is used to log HTTP requests and responses.
	Logger Logger

	// Ensures that the timeDelta function is only ran once
	// sync.Once would consider init done, even in case of error
	// hence a good old flag
	timeDeltaMutex *sync.Mutex
	timeDeltaDone  bool
	timeDelta      time.Duration
	Timeout        time.Duration

	// token used to generate api calls without credentials using OpenStack keystone
	openStackToken string
}

// NewClient represents a new client to call the API
func NewClient(endpoint, appKey, appSecret string, tenantid string) (*Client, error) {
	client := Client{
		AppKey:         appKey,
		AppSecret:      appSecret,
		Client:         &http.Client{},
		timeDeltaMutex: &sync.Mutex{},
		timeDeltaDone:  false,
		Timeout:        time.Duration(DefaultTimeout),
	}

	// Get and check the configuration
	if err := client.loadConfig(endpoint); err != nil {
		return nil, err
	}
	return &client, nil
}

// NewEndpointClient will create an API client for specified
// endpoint and load all credentials from environment or
// configuration files
func NewEndpointClient(endpoint string) (*Client, error) {
	return NewClient(endpoint, "", "", "")
}

// NewDefaultClient will load all it's parameter from environment
// or configuration files
func NewDefaultClient() (*Client, error) {
	return NewClient("", "", "", "")
}

// NewDefaultClientWithToken will load all it's parameter from environment
// or configuration files using an OpenStack keystone token
func NewDefaultClientWithToken(authUrl, token string) (*Client, error) {
	// Find endpoint given the keystone auth url
	endpoint := VKE
	client, err := NewClient(endpoint, "none", "none", "none")
	if err != nil {
		return nil, err
	}

	client.openStackToken = token

	return client, nil
}

// High level helpers
//
// In fact, ping is just a /auth/time call, in order to check if API is up.
func (c *Client) Ping() error {
	_, err := c.getTime()
	return err
}

// TimeDelta represents the delay between the machine that runs the code and the
func (c *Client) TimeDelta() (time.Duration, error) {
	return c.getTimeDelta()
}

func (c *Client) Time() (*time.Time, error) {
	return c.getTime()
}

//
// Common request wrappers
//

// Get is a wrapper for the GET method
func (c *Client) Get(url string, result interface{}, queryParams url.Values) error {
	return c.CallAPI("GET", url, nil, result, queryParams, true)
}

// GetUnAuth is a wrapper for the unauthenticated GET method
func (c *Client) GetUnAuth(url string, result interface{}, queryParams url.Values) error {
	return c.CallAPI("GET", url, nil, result, queryParams, false)
}

// Post is a wrapper for the POST method
func (c *Client) Post(url string, reqBody, result interface{}, queryParams url.Values) error {
	return c.CallAPI("POST", url, reqBody, result, queryParams, true)
}

// PostUnAuth is a wrapper for the unauthenticated POST method
func (c *Client) PostUnAuth(url string, reqBody, result interface{}, queryParams url.Values) error {
	return c.CallAPI("POST", url, reqBody, result, queryParams, false)
}

// Put is a wrapper for the PUT method
func (c *Client) Put(url string, reqBody, result interface{}, queryParams url.Values) error {
	return c.CallAPI("PUT", url, reqBody, result, queryParams, true)
}

// PutUnAuth is a wrapper for the unauthenticated PUT method
func (c *Client) PutUnAuth(url string, reqBody, result interface{}, queryParams url.Values) error {
	return c.CallAPI("PUT", url, reqBody, result, queryParams, false)
}

// Delete is a wrapper for the DELETE method
func (c *Client) Delete(url string, result interface{}, queryParams url.Values) error {
	return c.CallAPI("DELETE", url, nil, result, queryParams, true)
}

// DeleteUnAuth is a wrapper for the unauthenticated DELETE method
func (c *Client) DeleteUnAuth(url string, result interface{}, queryParams url.Values) error {
	return c.CallAPI("DELETE", url, nil, result, queryParams, false)
}

// GetWithContext is a wrapper for the GET method
func (c *Client) GetWithContext(ctx context.Context, url string, result interface{}, queryParams url.Values) error {
	return c.CallAPIWithContext(ctx, "GET", url, nil, result, queryParams, nil, true)
}

// GetUnAuthWithContext is a wrapper for the unauthenticated GET method
func (c *Client) GetUnAuthWithContext(ctx context.Context, url string, result interface{}, queryParams url.Values) error {
	return c.CallAPIWithContext(ctx, "GET", url, nil, result, queryParams, nil, false)
}

// PostWithContext is a wrapper for the POST method
func (c *Client) PostWithContext(ctx context.Context, url string, reqBody, result interface{}, queryParams url.Values) error {
	return c.CallAPIWithContext(ctx, "POST", url, reqBody, result, queryParams, nil, true)
}

// PostUnAuthWithContext is a wrapper for the unauthenticated POST method
func (c *Client) PostUnAuthWithContext(ctx context.Context, url string, reqBody, result interface{}, queryParams url.Values) error {
	return c.CallAPIWithContext(ctx, "POST", url, reqBody, result, queryParams, nil, false)
}

// PutWithContext is a wrapper for the PUT method
func (c *Client) PutWithContext(ctx context.Context, url string, reqBody, result interface{}, queryParams url.Values) error {
	return c.CallAPIWithContext(ctx, "PUT", url, reqBody, result, queryParams, nil, true)
}

// PutUnAuthWithContext is a wrapper for the unauthenticated PUT method
func (c *Client) PutUnAuthWithContext(ctx context.Context, url string, reqBody, result interface{}, queryParams url.Values) error {
	return c.CallAPIWithContext(ctx, "PUT", url, reqBody, result, queryParams, nil, false)
}

// DeleteWithContext is a wrapper for the DELETE method
func (c *Client) DeleteWithContext(ctx context.Context, url string, result interface{}, queryParams url.Values) error {
	return c.CallAPIWithContext(ctx, "DELETE", url, nil, result, queryParams, nil, true)
}

// DeleteUnAuthWithContext is a wrapper for the unauthenticated DELETE method
func (c *Client) DeleteUnAuthWithContext(ctx context.Context, url string, result interface{}, queryParams url.Values) error {
	return c.CallAPIWithContext(ctx, "DELETE", url, nil, result, queryParams, nil, false)
}

// timeDelta returns the time  delta between the host and the remote API
func (c *Client) getTimeDelta() (time.Duration, error) {
	if !c.timeDeltaDone {
		// Ensure only one thread is updating
		c.timeDeltaMutex.Lock()

		// Ensure that the mutex will be released on return
		defer c.timeDeltaMutex.Unlock()

		// Did we wait ? Maybe no more needed
		if !c.timeDeltaDone {
			vkeTime, err := c.getTime()
			if err != nil {
				return 0, err
			}

			c.timeDelta = time.Since(*vkeTime)
			c.timeDeltaDone = true
		}
	}

	return c.timeDelta, nil
}

// getTime t returns time from for a given api client endpoint
func (c *Client) getTime() (*time.Time, error) {
	var timestamp int64

	err := c.GetUnAuth("/auth/time", &timestamp, nil)
	if err != nil {
		return nil, err
	}

	serverTime := time.Unix(timestamp, 0)
	return &serverTime, nil
}

// getLocalTime is a function to be overwritten during the tests, it return the time
// on the the local machine
var getLocalTime = func() time.Time {
	return time.Now()
}

// getEndpointForSignature is a function to be overwritten during the tests, it returns a
// the endpoint
var getEndpointForSignature = func(c *Client) string {
	return c.endpoint
}

// NewRequest returns a new HTTP request
func (c *Client) NewRequest(method, path string, reqBody interface{}, queryParams url.Values, headers map[string]interface{}, needAuth bool) (*http.Request, error) {
	var body []byte
	var err error

	if reqBody != nil {
		body, err = json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}
	}

	target := fmt.Sprintf("%s%s", c.endpoint, path)
	req, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Inject headers
	if body != nil {
		req.Header.Add("Content-Type", "application/json;charset=utf-8")
	}
	req.Header.Add("Accept", "application/json")

	// Bind OpenStack token to authorization bearer and custom headers
	if c.openStackToken != "" {
		req.Header.Add("X-Auth-Token", fmt.Sprintf(c.openStackToken))
	}

	for headerName, headerValue := range headers {
		req.Header.Set(headerName, fmt.Sprintf("%v", headerValue))
	}

	// Inject signature. Some methods do not need authentication, especially /time,
	// /auth and some /order methods are actually broken if authenticated.
	if c.openStackToken == "" {
		timeDelta, err := c.TimeDelta()
		if err != nil {
			return nil, err
		}

		timestamp := getLocalTime().Add(-timeDelta).Unix()

		h := sha1.New()
		h.Write([]byte(fmt.Sprintf("%s+%s+%s+%s%s+%d",
			c.AppSecret,
			method,
			getEndpointForSignature(c),
			path,
			body,
			timestamp,
		)))
	}

	// Send the request with requested timeout
	c.Client.Timeout = c.Timeout

	return req, nil
}

// Do sends an HTTP request and returns an HTTP response
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.Logger != nil {
		c.Logger.LogRequest(req)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if c.Logger != nil {
		c.Logger.LogResponse(resp)
	}
	return resp, nil
}

// CallAPI is the lowest level call helper. If needAuth is true,
// inject authentication headers and sign the request.
//
// Request signature is a sha1 hash on following fields, joined by '+':
// - applicationSecret (from Client instance)
// - consumerKey (from Client instance)
// - capitalized method (from arguments)
// - full request url, including any query string argument
// - full serialized request body
// - server current time (takes time delta into account)
//
// Call will automatically assemble the target url from the endpoint
// configured in the client instance and the path argument. If the reqBody
// argument is not nil, it will also serialize it as json and inject
// the required Content-Type header.
//
// If everything went fine, unmarshall response into result and return nil
// otherwise, return the error
func (c *Client) CallAPI(method, path string, reqBody, result interface{}, queryParams url.Values, needAuth bool) error {
	return c.CallAPIWithContext(context.Background(), method, path, reqBody, result, queryParams, nil, needAuth)
}

// CallAPIWithContext is the lowest level call helper. If needAuth is true,
// inject authentication headers and sign the request.
//
// Request signature is a sha1 hash on following fields, joined by '+':
// - applicationSecret (from Client instance)
// - consumerKey (from Client instance)
// - capitalized method (from arguments)
// - full request url, including any query string argument
// - full serialized request body
// - server current time (takes time delta into account)
//
// # Context is used by http.Client to handle context cancelation
//
// Call will automatically assemble the target url from the endpoint
// configured in the client instance and the path argument. If the reqBody
// argument is not nil, it will also serialize it as json and inject
// the required Content-Type header.
//
// If everything went fine, unmarshall response into result and return nil
// otherwise, return the error
func (c *Client) CallAPIWithContext(ctx context.Context, method, path string, reqBody, result interface{}, queryParams url.Values, headers map[string]interface{}, needAuth bool) error {
	req, err := c.NewRequest(method, path, reqBody, queryParams, headers, needAuth)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	response, err := c.Do(req)
	if err != nil {
		return err
	}
	err = c.UnmarshalResponse(response, result)
	if err != nil {
		// This is a temporary fix until the issue is correctly handled
		if IsPossiblyCanadianTenantSyncError(err, req.URL.String()) {
			// Create a canadian API client with the same token
			client, err2 := NewClient(VKE, "none", "none", "")
			if err2 != nil {
				return fmt.Errorf("failed to create canadian VKE API client for fallback: %w", err2)
			}
			client.openStackToken = c.openStackToken

			err2 = client.CallAPIWithContext(ctx, method, path, reqBody, result, queryParams, headers, needAuth)
			if err2 == nil {
				// OK on the canadian API, our job is done
				return nil
			}
		}
	}

	return err
}

// UnmarshalResponse checks the response and unmarshals it into the response
// type if needed Helper function, called from CallAPI
func (c *Client) UnmarshalResponse(response *http.Response, result interface{}) error {
	// Read all the response body
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// < 200 && >= 300 : API error
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		apiError := &APIError{Code: response.StatusCode}
		if err = json.Unmarshal(body, apiError); err != nil {
			apiError.Message = string(body)
		}
		apiError.QueryID = response.Header.Get("X-VKE-QueryID")

		return apiError
	}

	// Nothing to unmarshal
	if len(body) == 0 || result == nil {
		return nil
	}

	return json.Unmarshal(body, &result)
}
