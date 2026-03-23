package core

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// FritzBoxFetcher fetches router status from a FritzBox via TR-064 UPnP.
// This works without authentication from the local LAN.
type FritzBoxFetcher struct {
	BaseURL string // e.g. "http://fritz.box:49000"
}

// Fetch queries the FritzBox TR-064 interface for link properties.
func (f *FritzBoxFetcher) Fetch() (*RouterStatus, error) {
	down, up, err := f.getLinkProperties()
	if err != nil {
		return nil, fmt.Errorf("fritzbox: getLinkProperties: %w", err)
	}

	status := &RouterStatus{
		DSLDownMbit: float64(down) / 1_000_000,
		DSLUpMbit:   float64(up) / 1_000_000,
		DSLOnline:   down > 0,
		LTEActive:   false,
		Mode:        "DSL",
		Online:      down > 0,
	}

	return status, nil
}

// soapBody wraps the SOAP envelope for a TR-064 request.
const soapEnvelopeFmt = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:%s xmlns:u="%s"/>
  </s:Body>
</s:Envelope>`

func (f *FritzBoxFetcher) soapCall(path, serviceType, action string) ([]byte, error) {
	url := f.BaseURL + path
	body := fmt.Sprintf(soapEnvelopeFmt, action, serviceType)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, serviceType, action))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// getLinkProperties returns downstream and upstream in bit/s.
func (f *FritzBoxFetcher) getLinkProperties() (down, up uint64, err error) {
	const (
		path        = "/igdupnp/control/WANCommonIFC1"
		serviceType = "urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1"
		action      = "GetCommonLinkProperties"
	)

	data, err := f.soapCall(path, serviceType, action)
	if err != nil {
		return 0, 0, err
	}

	// Parse the XML response generically – we only need two fields.
	var envelope struct {
		Body struct {
			Response struct {
				DownstreamMaxBitRate string `xml:"NewLayer1DownstreamMaxBitRate"`
				UpstreamMaxBitRate   string `xml:"NewLayer1UpstreamMaxBitRate"`
			} `xml:"GetCommonLinkPropertiesResponse"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(data, &envelope); err != nil {
		return 0, 0, fmt.Errorf("xml parse: %w", err)
	}

	downVal, _ := strconv.ParseUint(strings.TrimSpace(envelope.Body.Response.DownstreamMaxBitRate), 10, 64)
	upVal, _ := strconv.ParseUint(strings.TrimSpace(envelope.Body.Response.UpstreamMaxBitRate), 10, 64)

	return downVal, upVal, nil
}
