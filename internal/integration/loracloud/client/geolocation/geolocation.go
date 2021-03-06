package geolocation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	// "github.com/ibrahimozekici/lora-api/go/v3/common"
	// "github.com/ibrahimozekici/lora-api/go/v3/gw"
)

const (
	tdoaSingleFrameEndpoint       = `%s/api/v2/tdoa`
	tdoaMultiFrameEndpoint        = `%s/api/v2/tdoaMultiframe`
	rssiSingleFrameEndpoint       = `%s/api/v2/rssi`
	rssiMultiFrameEndpoint        = `%s/api/v2/rssiMultiframe`
	wifiTDOASingleFrameEndpoint   = `%s/api/v2/loraWifi`
	gnssLR1110SingleFrameEndpoint = `%s/api/v3/solve/gnss_lr1110_singleframe`
)

// errors
var (
	ErrNoLocation = errors.New("no location returned")
)

// Client is a LoRa Cloud Geolocation client.
type Client struct {
	uri            string
	token          string
	requestTimeout time.Duration
}

// New creates a new Geolocation client.
func New(uri string, token string) *Client {
	return &Client{
		uri:            uri,
		token:          token,
		requestTimeout: time.Second,
	}
}

// TDOASingleFrame request.
func (c *Client) TDOASingleFrame(ctx context.Context, rxInfo []*gw.UplinkRXInfo) (common.Location, error) {
	req := NewTDOASingleFrameRequest(rxInfo)
	resp, err := c.apiRequest(ctx, tdoaSingleFrameEndpoint, req)
	if err != nil {
		return common.Location{}, errors.Wrap(err, "api request error")
	}

	return c.parseResponse(resp, common.LocationSource_GEO_RESOLVER_TDOA)
}

// TDOAMultiFrame request.
func (c *Client) TDOAMultiFrame(ctx context.Context, rxInfo [][]*gw.UplinkRXInfo) (common.Location, error) {
	req := NewTDOAMultiFrameRequest(rxInfo)
	resp, err := c.apiRequest(ctx, tdoaMultiFrameEndpoint, req)
	if err != nil {
		return common.Location{}, errors.Wrap(err, "api request error")
	}

	return c.parseResponse(resp, common.LocationSource_GEO_RESOLVER_TDOA)
}

// RSSISingleFrame request.
func (c *Client) RSSISingleFrame(ctx context.Context, rxInfo []*gw.UplinkRXInfo) (common.Location, error) {
	req := NewRSSISingleFrameRequest(rxInfo)
	resp, err := c.apiRequest(ctx, rssiSingleFrameEndpoint, req)
	if err != nil {
		return common.Location{}, errors.Wrap(err, "api request error")
	}

	return c.parseResponse(resp, common.LocationSource_GEO_RESOLVER_RSSI)
}

// RSSIMultiFrame request.
func (c *Client) RSSIMultiFrame(ctx context.Context, rxInfo [][]*gw.UplinkRXInfo) (common.Location, error) {
	req := NewRSSIMultiFrameRequest(rxInfo)
	resp, err := c.apiRequest(ctx, rssiMultiFrameEndpoint, req)
	if err != nil {
		return common.Location{}, errors.Wrap(err, "api request error")
	}

	return c.parseResponse(resp, common.LocationSource_GEO_RESOLVER_RSSI)
}

// WifiTDOASingleFrame request.
func (c *Client) WifiTDOASingleFrame(ctx context.Context, rxInfo []*gw.UplinkRXInfo, aps []WifiAccessPoint) (common.Location, error) {
	req := NewWifiTDOASingleFrameRequest(rxInfo, aps)
	resp, err := c.apiRequest(ctx, wifiTDOASingleFrameEndpoint, req)
	if err != nil {
		return common.Location{}, errors.Wrap(err, "api request error")
	}

	return c.parseResponse(resp, common.LocationSource_GEO_RESOLVER_WIFI)
}

// GNSSLR1110SingleFrame request.
func (c *Client) GNSSLR1110SingleFrame(ctx context.Context, rxInfo []*gw.UplinkRXInfo, useRxTime bool, pl []byte) (common.Location, error) {
	req := NewGNSSLR1110SingleFrameRequest(rxInfo, useRxTime, pl)
	resp, err := c.v3APIRequest(ctx, gnssLR1110SingleFrameEndpoint, req)
	if err != nil {
		return common.Location{}, errors.Wrap(err, "api request error")
	}

	return c.parseV3Response(resp, common.LocationSource_GEO_RESOLVER_GNSS)
}

func (c *Client) parseResponse(resp Response, source common.LocationSource) (common.Location, error) {
	if len(resp.Errors) != 0 {
		return common.Location{}, fmt.Errorf("api returned error(s): %s", strings.Join(resp.Errors, ", "))
	}

	if resp.Result == nil {
		return common.Location{}, ErrNoLocation
	}

	return common.Location{
		Latitude:  resp.Result.Latitude,
		Longitude: resp.Result.Longitude,
		Altitude:  resp.Result.Altitude,
		Accuracy:  uint32(resp.Result.Accuracy),
		Source:    source,
	}, nil
}

func (c *Client) parseV3Response(resp V3Response, source common.LocationSource) (common.Location, error) {
	if len(resp.Errors) != 0 {
		return common.Location{}, fmt.Errorf("api returned error(s): %s", strings.Join(resp.Errors, ", "))
	}

	if resp.Result == nil {
		return common.Location{}, ErrNoLocation
	}

	if len(resp.Result.LLH) != 3 {
		return common.Location{}, fmt.Errorf("LLH must contain 3 items, received: %d", len(resp.Result.LLH))
	}

	return common.Location{
		Source:    source,
		Latitude:  resp.Result.LLH[0],
		Longitude: resp.Result.LLH[1],
		Altitude:  resp.Result.LLH[2],
		Accuracy:  uint32(resp.Result.Accuracy),
	}, nil
}

func (c *Client) apiRequest(ctx context.Context, endpoint string, v interface{}) (Response, error) {
	endpoint = fmt.Sprintf(endpoint, c.uri)
	var resp Response

	b, err := json.Marshal(v)
	if err != nil {
		return resp, errors.Wrap(err, "json marshal error")
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(b))
	if err != nil {
		return resp, errors.Wrap(err, "create request error")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Ocp-Apim-Subscription-Key", c.token)

	reqCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	req = req.WithContext(reqCtx)
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, errors.Wrap(err, "http request error")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		bb, _ := ioutil.ReadAll(httpResp.Body)
		return resp, fmt.Errorf("expected 200, got: %d (%s)", httpResp.StatusCode, string(bb))
	}

	if err = json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return resp, errors.Wrap(err, "unmarshal response error")
	}

	return resp, nil
}

func (c *Client) v3APIRequest(ctx context.Context, endpoint string, v interface{}) (V3Response, error) {
	endpoint = fmt.Sprintf(endpoint, c.uri)
	var resp V3Response

	b, err := json.Marshal(v)
	if err != nil {
		return resp, errors.Wrap(err, "json marshal error")
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(b))
	if err != nil {
		return resp, errors.Wrap(err, "create request error")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Ocp-Apim-Subscription-Key", c.token)

	reqCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	req = req.WithContext(reqCtx)
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, errors.Wrap(err, "http request error")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		bb, _ := ioutil.ReadAll(httpResp.Body)
		return resp, fmt.Errorf("expected 200, got: %d (%s)", httpResp.StatusCode, string(bb))
	}

	if err = json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return resp, errors.Wrap(err, "unmarshal response error")
	}

	return resp, nil
}
