package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ICSClient is the API client for Ingenuity Cloud Services
type ICSClient struct {
	APIToken   string
	BaseURL    string
	HTTPClient *http.Client
}

// APIResponse represents the standard API response format
type APIResponse struct {
	StatusCode int         `json:"statusCode"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
}

// InventoryItem represents a server SKU in inventory
type InventoryItem struct {
	SkuID                 int                    `json:"sku_id"`
	Quantity              int                    `json:"quantity"`
	AutoProvisionQuantity int                    `json:"auto_provision_quantity"`
	DatacenterID          int                    `json:"datacenter_id"`
	RegionID              int                    `json:"region_id"`
	LocationCode          string                 `json:"location_code"`
	CPUBrand              string                 `json:"cpu_brand"`
	CPUModel              string                 `json:"cpu_model"`
	CPUClockSpeedGHz      float64                `json:"cpu_clock_speed_ghz"`
	CPUCores              int                    `json:"cpu_cores"`
	CPUCount              int                    `json:"cpu_count"`
	TotalSSDSizeGB        int                    `json:"total_ssd_size_gb"`
	TotalHDDSizeGB        int                    `json:"total_hdd_size_gb"`
	TotalNVMESizeGB       int                    `json:"total_nvme_size_gb"`
	RAIDEnabled           bool                   `json:"raid_enabled"`
	TotalRAMGB            int                    `json:"total_ram_gb"`
	NICSpeedMbps          int                    `json:"nic_speed_mbps"`
	QTProductID           int                    `json:"qt_product_id"`
	Status                string                 `json:"status"`
	Metadata              []InventoryMetadata    `json:"metadata"`
	CurrencyCode          string                 `json:"currency_code"`
	SkuProductName        string                 `json:"sku_product_name"`
	Price                 string                 `json:"price"`
	PriceHourly           string                 `json:"price_hourly"`
	HourlyEnabled         bool                   `json:"hourly_enabled"`
}

// InventoryMetadata represents metadata for an inventory item
type InventoryMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

// Server represents a bare metal server
type Server struct {
	ID                 string `json:"id"`
	Hostname           string `json:"hostname"`
	MacAddress         string `json:"mac_address"`
	PublicIP           string `json:"public_ip"`
	ServiceID          int    `json:"service_id"`
	ServiceDescription string `json:"service_description"`
	PlanID             int    `json:"plan_id"`
	DatacenterName     string `json:"datacenter_name"`
	DatacenterID       int    `json:"datacenter_id"`
	LocationID         int    `json:"location_id"`
	FriendlyName       string `json:"friendly_name"`
	Vendor             string `json:"vendor"`
	ServerType         string `json:"server_type"`
	BillHourly         bool   `json:"bill_hourly"`
	RootPassword       string `json:"root_password"`
}

// ServerOrderRequest represents a server order request
type ServerOrderRequest struct {
	SkuProductName              string   `json:"sku_product_name"`              // Required
	Quantity                    int      `json:"quantity"`                       // Required
	LocationCode                string   `json:"location_code"`                  // Required
	OperatingSystemProductCode  string   `json:"operating_system_product_code"`  // Required
	Hostname                    string   `json:"hostname,omitempty"`
	BillHourly                  bool     `json:"bill_hourly,omitempty"`
	SSHKeyIDs                   []int    `json:"ssh_key_ids,omitempty"`
}

// ServerOrderResponse represents the response from ordering a server
type ServerOrderResponse struct {
	OrderServiceIDs []int `json:"order_service_ids"`
}

// AddonsResponse represents the response from the addons API
type AddonsResponse struct {
	OperatingSystems OperatingSystemsAddon `json:"operating_systems"`
	Licenses         LicensesAddon         `json:"licenses"`
	SupportLevels    SupportLevelsAddon    `json:"support_levels"`
}

// OperatingSystemsAddon represents available operating systems
type OperatingSystemsAddon struct {
	Name     string                `json:"name"`
	Required string                `json:"required"`
	Products []OperatingSystemItem `json:"products"`
}

// OperatingSystemItem represents a single operating system option
type OperatingSystemItem struct {
	Name          string  `json:"name"`
	OSType        string  `json:"os_type"`
	ProductCode   string  `json:"product_code"`
	Price         float64 `json:"price"`
	PricePerCore  *float64 `json:"price_per_core"`
	PriceHourly   float64 `json:"price_hourly"`
	HourlyEnabled bool    `json:"hourly_enabled"`
}

// LicensesAddon represents available licenses
type LicensesAddon struct {
	Name     string        `json:"name"`
	Products []LicenseItem `json:"products"`
}

// LicenseItem represents a single license option
type LicenseItem struct {
	Name          string  `json:"name"`
	ProductCode   string  `json:"product_code"`
	Price         float64 `json:"price"`
	PriceHourly   float64 `json:"price_hourly"`
	HourlyEnabled bool    `json:"hourly_enabled"`
}

// SupportLevelsAddon represents available support levels
type SupportLevelsAddon struct {
	Name     string           `json:"name"`
	Products []SupportItem    `json:"products"`
}

// SupportItem represents a single support level option
type SupportItem struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	ProductCode   string  `json:"product_code"`
	Price         float64 `json:"price"`
	PriceHourly   float64 `json:"price_hourly"`
	HourlyEnabled bool    `json:"hourly_enabled"`
}

// SSHKey represents an SSH key
type SSHKey struct {
	ID              int                 `json:"id"`
	Label           string              `json:"label"`
	Key             string              `json:"key"`
	CreatedAt       int64               `json:"created_at"`
	UpdatedAt       int64               `json:"updated_at"`
	AssignedServers []AssignedServer    `json:"assigned_servers"`
}

// AssignedServer represents a server assigned to an SSH key
type AssignedServer struct {
	ServerID       string `json:"server_id"`
	ServiceID      int    `json:"service_id"`
	Hostname       string `json:"hostname"`
	DatacenterName string `json:"datacenter_name"`
}

// SSHKeyCreateRequest represents a request to create an SSH key
type SSHKeyCreateRequest struct {
	PublicKey string `json:"public_key"`
	Label     string `json:"label"`
}

// SSHKeyCreateResponse represents the response from creating an SSH key
type SSHKeyCreateResponse struct {
	ID int `json:"id"`
}

// FriendlyNameUpdateRequest represents a request to update server friendly name
type FriendlyNameUpdateRequest struct {
	FriendlyName string `json:"friendly_name"`
}

// NewICSClient creates a new ICS API client. baseURL must be an absolute
// http(s) URL so that requests can only ever be sent to the configured host.
func NewICSClient(apiToken, baseURL string) (*ICSClient, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" ||
		u.User != nil || strings.ContainsAny(baseURL, "?#") {
		// The value is not echoed back because it may contain credentials.
		return nil, errors.New("invalid base URL: must be an absolute http(s) URL with a host and without credentials, query or fragment")
	}

	baseOrigin := origin(u)
	return &ICSClient{
		APIToken: apiToken,
		BaseURL:  strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 300 * time.Second, // Increased to 5 minutes for server ordering
			// Go forwards custom headers such as X-Api-Token on redirects, so
			// refuse any redirect that leaves the configured origin.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if origin(req.URL) != baseOrigin {
					return fmt.Errorf("refusing redirect to %s://%s outside the base URL origin", req.URL.Scheme, req.URL.Host)
				}
				if len(via) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
				return nil
			},
		},
	}, nil
}

// origin returns the scheme, lower-cased host and effective port of u, so that
// e.g. https://Example.com:443 and https://example.com compare equal.
func origin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	return u.Scheme + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

// serverPath returns the API path for a server sub-resource. The ID is escaped
// and dot segments are rejected so it always stays a single path segment.
func serverPath(serverID, action string) (string, error) {
	if serverID == "" || serverID == "." || serverID == ".." {
		return "", fmt.Errorf("invalid server ID %q", serverID)
	}
	return "/rest-api/servers/" + url.PathEscape(serverID) + "/" + action, nil
}

// makeRequest makes an HTTP request to the ICS API
func (c *ICSClient) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, c.BaseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Token", c.APIToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.HTTPClient.Do(req)
}

// GetInventory retrieves the server inventory
func (c *ICSClient) GetInventory() ([]InventoryItem, error) {
	resp, err := c.makeRequest("GET", "/rest-api/server-orders/inventory", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert the data interface{} to []InventoryItem
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var inventory []InventoryItem
	if err := json.Unmarshal(dataBytes, &inventory); err != nil {
		return nil, fmt.Errorf("failed to unmarshal inventory data: %w", err)
	}

	return inventory, nil
}

// OrderServer orders a new bare metal server
func (c *ICSClient) OrderServer(request ServerOrderRequest) (*ServerOrderResponse, error) {
	resp, err := c.makeRequest("POST", "/rest-api/server-orders/order", request)
	if err != nil {
		return nil, fmt.Errorf("failed to order server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert the data interface{} to ServerOrderResponse
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var orderResp ServerOrderResponse
	if err := json.Unmarshal(dataBytes, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order response data: %w", err)
	}

	return &orderResp, nil
}

// GetServers retrieves all servers
func (c *ICSClient) GetServers() ([]Server, error) {
	resp, err := c.makeRequest("GET", "/rest-api/servers", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get servers: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert the data interface{} to []Server
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var servers []Server
	if err := json.Unmarshal(dataBytes, &servers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal servers data: %w", err)
	}

	return servers, nil
}

// GetServerByServiceID retrieves a server by its service ID
func (c *ICSClient) GetServerByServiceID(serviceID int) (*Server, error) {
	servers, err := c.GetServers()
	if err != nil {
		return nil, err
	}

	for _, server := range servers {
		if server.ServiceID == serviceID {
			return &server, nil
		}
	}

	return nil, fmt.Errorf("server with service ID %d not found", serviceID)
}

// CancelServer cancels/deletes a server (hourly billed servers only)
func (c *ICSClient) CancelServer(serverID string) error {
	endpoint, err := serverPath(serverID, "cancel")
	if err != nil {
		return err
	}
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to cancel server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetAddons retrieves available addons for a specific SKU and location
func (c *ICSClient) GetAddons(skuProductName, locationCode string) (*AddonsResponse, error) {
	endpoint := "/rest-api/server-orders/list-addons?" + url.Values{
		"sku_product_name": {skuProductName},
		"location_code":    {locationCode},
	}.Encode()
	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get addons: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// The addons response contains a single object with the addon data
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var addonsResp AddonsResponse
	if err := json.Unmarshal(dataBytes, &addonsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal addons data: %w", err)
	}

	return &addonsResp, nil
}

// FindSKUByProductName finds a SKU by its product name and validates inventory
func (c *ICSClient) FindSKUByProductName(productName, locationCode string) (*InventoryItem, error) {
	inventory, err := c.GetInventory()
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	for _, item := range inventory {
		if item.SkuProductName == productName {
			// Check if location matches (if specified)
			if locationCode != "" && item.LocationCode != locationCode {
				continue
			}
			// Ensure we have auto provision quantity available
			if item.AutoProvisionQuantity > 0 {
				return &item, nil
			}
		}
	}

	if locationCode != "" {
		return nil, fmt.Errorf("SKU '%s' not found with auto provision inventory in location '%s'", productName, locationCode)
	}
	return nil, fmt.Errorf("SKU '%s' not found with auto provision inventory", productName)
}

// GetOperatingSystemByName finds an operating system by name for a specific SKU and location
func (c *ICSClient) GetOperatingSystemByName(skuProductName, locationCode, osName string) (*OperatingSystemItem, error) {
	addons, err := c.GetAddons(skuProductName, locationCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get addons: %w", err)
	}

	for _, os := range addons.OperatingSystems.Products {
		if os.Name == osName {
			return &os, nil
		}
	}

	return nil, fmt.Errorf("operating system '%s' not found for SKU '%s' in location '%s'", osName, skuProductName, locationCode)
}

// CreateSSHKey creates a new SSH key
func (c *ICSClient) CreateSSHKey(request SSHKeyCreateRequest) (*SSHKeyCreateResponse, error) {
	resp, err := c.makeRequest("POST", "/rest-api/ssh-keys", request)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH key: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert the data interface{} to SSHKeyCreateResponse
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var createResp SSHKeyCreateResponse
	if err := json.Unmarshal(dataBytes, &createResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SSH key create response data: %w", err)
	}

	return &createResp, nil
}

// GetSSHKeys retrieves all SSH keys
func (c *ICSClient) GetSSHKeys() ([]SSHKey, error) {
	resp, err := c.makeRequest("GET", "/rest-api/ssh-keys", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH keys: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert the data interface{} to []SSHKey
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var sshKeys []SSHKey
	if err := json.Unmarshal(dataBytes, &sshKeys); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SSH keys data: %w", err)
	}

	return sshKeys, nil
}

// GetSSHKeyByLabel finds an SSH key by its label
func (c *ICSClient) GetSSHKeyByLabel(label string) (*SSHKey, error) {
	sshKeys, err := c.GetSSHKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH keys: %w", err)
	}

	for _, key := range sshKeys {
		if key.Label == label {
			return &key, nil
		}
	}

	return nil, fmt.Errorf("SSH key with label '%s' not found", label)
}

// DeleteSSHKey deletes an SSH key by ID
func (c *ICSClient) DeleteSSHKey(keyID int) error {
	endpoint := fmt.Sprintf("/rest-api/ssh-keys/%d", keyID)
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to delete SSH key: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UpdateServerFriendlyName updates the friendly name of a server
func (c *ICSClient) UpdateServerFriendlyName(serverID, friendlyName string) error {
	endpoint, err := serverPath(serverID, "friendly-name")
	if err != nil {
		return err
	}

	request := FriendlyNameUpdateRequest{
		FriendlyName: friendlyName,
	}

	resp, err := c.makeRequest("PUT", endpoint, request)
	if err != nil {
		return fmt.Errorf("failed to update server friendly name: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}