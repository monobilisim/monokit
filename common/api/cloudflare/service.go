package cloudflare

import (
	"fmt"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Service provides Cloudflare domain management functionality
type Service struct {
	db           *gorm.DB
	globalClient *Client
	config       models.CloudflareConfig
}

// NewService creates a new Cloudflare service
func NewService(db *gorm.DB, config models.CloudflareConfig) *Service {
	var globalClient *Client
	if config.Enabled && config.APIToken != "" {
		client, err := NewClient(config)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create global Cloudflare client")
		} else {
			globalClient = client
		}
	}

	return &Service{
		db:           db,
		globalClient: globalClient,
		config:       config,
	}
}

// GetClient returns a Cloudflare client for a specific domain
// If the domain has its own API token, it uses that; otherwise, it uses the global client
func (s *Service) GetClient(cfDomain *models.CloudflareDomain) (*Client, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("Cloudflare integration is not enabled")
	}

	// Use domain-specific token if available
	if cfDomain.APIToken != "" {
		timeout := s.config.Timeout
		if timeout == 0 {
			timeout = 30 // default timeout
		}
		return NewClientWithToken(cfDomain.APIToken, timeout)
	}

	// Use global client
	if s.globalClient == nil {
		return nil, fmt.Errorf("no global Cloudflare client available and domain has no specific token")
	}

	return s.globalClient, nil
}

// CreateCloudflareDomain creates a new Cloudflare domain configuration
func (s *Service) CreateCloudflareDomain(domainID uint, req models.CreateCloudflareDomainRequest) (*models.CloudflareDomain, error) {
	// Create the CloudflareDomain record
	cfDomain := models.CloudflareDomain{
		DomainID:     domainID,
		ZoneName:     req.ZoneName,
		ZoneID:       req.ZoneID,
		APIToken:     req.APIToken,
		ProxyEnabled: true, // default to true
		Active:       true,
	}

	// Set proxy enabled if specified
	if req.ProxyEnabled != nil {
		cfDomain.ProxyEnabled = *req.ProxyEnabled
	}

	// Verify the zone exists and is accessible (only if Cloudflare is enabled)
	if s.config.Enabled {
		client, err := s.GetClient(&cfDomain)
		if err != nil {
			return nil, fmt.Errorf("failed to get Cloudflare client: %w", err)
		}

		if err := client.VerifyZone(req.ZoneID); err != nil {
			return nil, fmt.Errorf("failed to verify Cloudflare zone: %w", err)
		}
	}

	// Save to database
	if err := s.db.Create(&cfDomain).Error; err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare domain: %w", err)
	}

	log.Info().
		Uint("domain_id", domainID).
		Str("zone_name", req.ZoneName).
		Str("zone_id", req.ZoneID).
		Msg("Created Cloudflare domain configuration")

	return &cfDomain, nil
}

// UpdateCloudflareDomain updates an existing Cloudflare domain configuration
func (s *Service) UpdateCloudflareDomain(id uint, req models.UpdateCloudflareDomainRequest) (*models.CloudflareDomain, error) {
	var cfDomain models.CloudflareDomain
	if err := s.db.First(&cfDomain, id).Error; err != nil {
		return nil, fmt.Errorf("Cloudflare domain not found: %w", err)
	}

	// Update fields if provided
	if req.ZoneName != "" {
		cfDomain.ZoneName = req.ZoneName
	}
	if req.ZoneID != "" {
		cfDomain.ZoneID = req.ZoneID
	}
	if req.APIToken != "" {
		cfDomain.APIToken = req.APIToken
	}
	if req.ProxyEnabled != nil {
		cfDomain.ProxyEnabled = *req.ProxyEnabled
	}
	if req.Active != nil {
		cfDomain.Active = *req.Active
	}

	// Verify the zone if zone ID was changed (only if Cloudflare is enabled)
	if req.ZoneID != "" && s.config.Enabled {
		client, err := s.GetClient(&cfDomain)
		if err != nil {
			return nil, fmt.Errorf("failed to get Cloudflare client: %w", err)
		}

		if err := client.VerifyZone(cfDomain.ZoneID); err != nil {
			return nil, fmt.Errorf("failed to verify Cloudflare zone: %w", err)
		}
	}

	// Save changes
	if err := s.db.Save(&cfDomain).Error; err != nil {
		return nil, fmt.Errorf("failed to update Cloudflare domain: %w", err)
	}

	log.Info().
		Uint("id", id).
		Str("zone_name", cfDomain.ZoneName).
		Str("zone_id", cfDomain.ZoneID).
		Msg("Updated Cloudflare domain configuration")

	return &cfDomain, nil
}

// DeleteCloudflareDomain deletes a Cloudflare domain configuration
func (s *Service) DeleteCloudflareDomain(id uint) error {
	var cfDomain models.CloudflareDomain
	if err := s.db.First(&cfDomain, id).Error; err != nil {
		return fmt.Errorf("Cloudflare domain not found: %w", err)
	}

	if err := s.db.Delete(&cfDomain).Error; err != nil {
		return fmt.Errorf("failed to delete Cloudflare domain: %w", err)
	}

	log.Info().
		Uint("id", id).
		Str("zone_name", cfDomain.ZoneName).
		Msg("Deleted Cloudflare domain configuration")

	return nil
}

// GetCloudflareDomains retrieves all Cloudflare domains for a specific domain
func (s *Service) GetCloudflareDomains(domainID uint) ([]models.CloudflareDomain, error) {
	var cfDomains []models.CloudflareDomain
	if err := s.db.Where("domain_id = ?", domainID).Find(&cfDomains).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve Cloudflare domains: %w", err)
	}

	return cfDomains, nil
}

// GetCloudflareDomain retrieves a specific Cloudflare domain by ID
func (s *Service) GetCloudflareDomain(id uint) (*models.CloudflareDomain, error) {
	var cfDomain models.CloudflareDomain
	if err := s.db.First(&cfDomain, id).Error; err != nil {
		return nil, fmt.Errorf("Cloudflare domain not found: %w", err)
	}

	return &cfDomain, nil
}

// TestConnection tests the connection to Cloudflare API for a specific domain
func (s *Service) TestConnection(cfDomainID uint) error {
	if !s.config.Enabled {
		return fmt.Errorf("Cloudflare integration is not enabled")
	}

	cfDomain, err := s.GetCloudflareDomain(cfDomainID)
	if err != nil {
		return err
	}

	client, err := s.GetClient(cfDomain)
	if err != nil {
		return err
	}

	return client.TestConnection()
}

// GetZones retrieves all zones accessible by a Cloudflare domain configuration
func (s *Service) GetZones(cfDomainID uint) ([]Zone, error) {
	cfDomain, err := s.GetCloudflareDomain(cfDomainID)
	if err != nil {
		return nil, err
	}

	client, err := s.GetClient(cfDomain)
	if err != nil {
		return nil, err
	}

	return client.GetZones()
}

// GetDNSRecords retrieves DNS records for a Cloudflare domain
func (s *Service) GetDNSRecords(cfDomainID uint) ([]DNSRecord, error) {
	cfDomain, err := s.GetCloudflareDomain(cfDomainID)
	if err != nil {
		return nil, err
	}

	client, err := s.GetClient(cfDomain)
	if err != nil {
		return nil, err
	}

	return client.GetDNSRecords(cfDomain.ZoneID)
}

// CreateDNSRecord creates a DNS record for a Cloudflare domain
func (s *Service) CreateDNSRecord(cfDomainID uint, record DNSRecord) (*DNSRecord, error) {
	cfDomain, err := s.GetCloudflareDomain(cfDomainID)
	if err != nil {
		return nil, err
	}

	client, err := s.GetClient(cfDomain)
	if err != nil {
		return nil, err
	}

	// Set proxy based on domain configuration if not explicitly set
	if cfDomain.ProxyEnabled && (record.Type == "A" || record.Type == "AAAA" || record.Type == "CNAME") {
		record.Proxied = &cfDomain.ProxyEnabled
	}

	return client.CreateDNSRecord(cfDomain.ZoneID, record)
}

// UpdateDNSRecord updates a DNS record for a Cloudflare domain
func (s *Service) UpdateDNSRecord(cfDomainID uint, recordID string, record DNSRecord) (*DNSRecord, error) {
	cfDomain, err := s.GetCloudflareDomain(cfDomainID)
	if err != nil {
		return nil, err
	}

	client, err := s.GetClient(cfDomain)
	if err != nil {
		return nil, err
	}

	return client.UpdateDNSRecord(cfDomain.ZoneID, recordID, record)
}

// DeleteDNSRecord deletes a DNS record for a Cloudflare domain
func (s *Service) DeleteDNSRecord(cfDomainID uint, recordID string) error {
	cfDomain, err := s.GetCloudflareDomain(cfDomainID)
	if err != nil {
		return err
	}

	client, err := s.GetClient(cfDomain)
	if err != nil {
		return err
	}

	return client.DeleteDNSRecord(cfDomain.ZoneID, recordID)
}
