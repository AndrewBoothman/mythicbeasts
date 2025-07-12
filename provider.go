package mythicbeasts

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/libdns/libdns"
	"golang.org/x/net/publicsuffix"
)

// Provider facilitates DNS record manipulation with Mythic Beasts.
type Provider struct {
	KeyID  string `json:"key_id,omitempty"`
	Secret string `json:"secret,omitempty"`

	token mythicAuthResponse

	mutex sync.Mutex
}

// unFQDN trims any trailing "." from fqdn.
func (p *Provider) unFQDN(fqdn string) string {
	return strings.TrimSuffix(fqdn, ".")
}

// GetRecords lists all records in given zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	err := p.login(ctx)
	if err != nil {
		return nil, fmt.Errorf("login: provider login failed: %d", err)
	}

	formatedZone, err := publicsuffix.EffectiveTLDPlusOne(p.unFQDN(zone))
	if err != nil {
		return nil, fmt.Errorf("Provided zone string malformed %d", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL+"/zones/"+formatedZone+"/records", nil)
	if err != nil {
		return nil, fmt.Errorf("login: provider record request failed: %d", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.token.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login: provider record request failed: %d", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("login: failed to read response body: %d", err)
	}

	result := mythicRecords{}

	result.UnmarshalJSON(body)
	var records []libdns.Record

	for _, r := range result.Records {
		record, err := r.GetLibdnsRecord()
		if err != nil {
			return nil, fmt.Errorf("GetRecords: failed to parse record %s: %d", r.GetName(), err)
		}

		records = append(records, record)
	}
	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	err := p.login(ctx)
	if err != nil {
		return nil, fmt.Errorf("login: provider login failed: %d", err)
	}

	formatedZone, err := publicsuffix.EffectiveTLDPlusOne(p.unFQDN(zone))
	if err != nil {
		return nil, fmt.Errorf("Provided zone string malformed %d", err)
	}

	var appendedRecords []libdns.Record

	for _, record := range records {
		newRecord, err := p.addRecord(ctx, formatedZone, record)
		if err != nil {
			return nil, fmt.Errorf("AppendRecords: %d", err)
		}
		appendedRecords = append(appendedRecords, newRecord[0])
	}

	return appendedRecords, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	err := p.login(ctx)
	if err != nil {
		return nil, fmt.Errorf("login: provider login failed: %d", err)
	}

	formatedZone, err := publicsuffix.EffectiveTLDPlusOne(p.unFQDN(zone))
	if err != nil {
		return nil, fmt.Errorf("Provided zone string malformed %d", err)
	}

	var setRecords []libdns.Record

	for _, record := range records {
		setRecord, err := p.updateRecord(ctx, formatedZone, record)
		if err != nil {
			return setRecords, fmt.Errorf("SetRecords: %d", err)
		}
		setRecords = append(setRecords, setRecord...)
	}

	return setRecords, nil
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	err := p.login(ctx)
	if err != nil {
		return nil, fmt.Errorf("login: provider login failed: %d", err)
	}

	formatedZone, err := publicsuffix.EffectiveTLDPlusOne(p.unFQDN(zone))
	if err != nil {
		return nil, fmt.Errorf("Provided zone string malformed %d", err)
	}

	var deletedRecords []libdns.Record

	for _, record := range records {
		deletedRecord, err := p.removeRecord(ctx, formatedZone, record)
		if err != nil {
			return deletedRecords, fmt.Errorf("DeleteRecords: %d", err)
		}
		deletedRecords = append(deletedRecords, deletedRecord...)
	}

	return deletedRecords, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
