package cloudflare

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/rs/zerolog/log"
)

// Client wraps the official Cloudflare Go client
type Client struct {
	cf       *cloudflare.API
	timeout  time.Duration
	APIToken string // Store the API token for testing purposes
}

// CloudflareResponse represents a generic Cloudflare API response
type CloudflareResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result"`
	Errors  []string    `json:"errors,omitempty"`
}

// NewClient creates a new Cloudflare API client
func NewClient(config models.CloudflareConfig) (*Client, error) {
	var cf *cloudflare.API
	var err error

	if config.APIToken != "" {
		cf, err = cloudflare.NewWithAPIToken(config.APIToken)
	} else if config.APIKey != "" && config.Email != "" {
		cf, err = cloudflare.New(config.APIKey, config.Email)
	} else {
		return nil, fmt.Errorf("no valid authentication method provided")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare client: %w", err)
	}

	return &Client{
		cf:       cf,
		timeout:  time.Duration(config.Timeout) * time.Second,
		APIToken: config.APIToken,
	}, nil
}

// NewClientWithToken creates a new Cloudflare API client with a specific token
func NewClientWithToken(token string, timeout int) (*Client, error) {
	cf, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare client: %w", err)
	}

	return &Client{
		cf:       cf,
		timeout:  time.Duration(timeout) * time.Second,
		APIToken: token,
	}, nil
}

// Zone represents a Cloudflare zone (using cloudflare-go types)
type Zone = cloudflare.Zone

// DNSRecord represents a Cloudflare DNS record (using cloudflare-go types)
type DNSRecord = cloudflare.DNSRecord

// GetZones retrieves all zones for the account
func (c *Client) GetZones() ([]Zone, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	zones, err := c.cf.ListZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}

	return zones, nil
}

// GetZone retrieves a specific zone by ID
func (c *Client) GetZone(zoneID string) (*Zone, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	zone, err := c.cf.ZoneDetails(ctx, zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone details: %w", err)
	}

	return &zone, nil
}

// GetZoneByName retrieves a zone by its name
func (c *Client) GetZoneByName(zoneName string) (*Zone, error) {
	zoneID, err := c.cf.ZoneIDByName(zoneName)
	if err != nil {
		return nil, fmt.Errorf("zone %s not found: %w", zoneName, err)
	}

	return c.GetZone(zoneID)
}

// VerifyZone verifies that a zone exists and is accessible
func (c *Client) VerifyZone(zoneID string) error {
	_, err := c.GetZone(zoneID)
	return err
}

// TestConnection tests the connection to Cloudflare API
func (c *Client) TestConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Try to verify token by getting user details
	_, err := c.cf.UserDetails(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Cloudflare API connection test failed")
		return fmt.Errorf("failed to verify Cloudflare API connection: %w", err)
	}

	log.Info().Msg("Cloudflare API connection test successful")
	return nil
}

// GetDNSRecords retrieves DNS records for a zone
func (c *Client) GetDNSRecords(zoneID string) ([]DNSRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	rcs := cloudflare.ResourceContainer{
		Identifier: zoneID,
	}

	records, _, err := c.cf.ListDNSRecords(ctx, &rcs, cloudflare.ListDNSRecordsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list DNS records: %w", err)
	}

	return records, nil
}

// CreateDNSRecord creates a new DNS record
func (c *Client) CreateDNSRecord(zoneID string, record DNSRecord) (*DNSRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	rcs := cloudflare.ResourceContainer{
		Identifier: zoneID,
	}

	params := cloudflare.CreateDNSRecordParams{
		Type:    record.Type,
		Name:    record.Name,
		Content: record.Content,
		TTL:     record.TTL,
	}

	if record.Proxied != nil {
		params.Proxied = record.Proxied
	}

	createdRecord, err := c.cf.CreateDNSRecord(ctx, &rcs, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create DNS record: %w", err)
	}

	return &createdRecord, nil
}

// UpdateDNSRecord updates an existing DNS record
func (c *Client) UpdateDNSRecord(zoneID, recordID string, record DNSRecord) (*DNSRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	rcs := cloudflare.ResourceContainer{
		Identifier: zoneID,
	}

	params := cloudflare.UpdateDNSRecordParams{
		ID:      recordID,
		Type:    record.Type,
		Name:    record.Name,
		Content: record.Content,
		TTL:     record.TTL,
	}

	if record.Proxied != nil {
		params.Proxied = record.Proxied
	}

	updatedRecord, err := c.cf.UpdateDNSRecord(ctx, &rcs, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update DNS record: %w", err)
	}

	return &updatedRecord, nil
}

// DeleteDNSRecord deletes a DNS record
func (c *Client) DeleteDNSRecord(zoneID, recordID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	rcs := cloudflare.ResourceContainer{
		Identifier: zoneID,
	}

	err := c.cf.DeleteDNSRecord(ctx, &rcs, recordID)
	if err != nil {
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	return nil
}
