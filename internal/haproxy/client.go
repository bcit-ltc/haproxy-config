package haproxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"gopkg.in/yaml.v3"
)

// APIConfig holds HAProxy Dataplane API configuration
type APIConfig struct {
	URL      string
	Username string
	Password string
	Insecure bool
}

// Config represents the HAProxy configuration from ConfigMap
type Config struct {
	APIConfig APIConfigSpec `yaml:"apiConfig"`
	Backends  []Backend     `yaml:"backends"`
	Frontends []Frontend    `yaml:"frontends"`
}

// APIConfigSpec specifies how to connect to HAProxy API
type APIConfigSpec struct {
	URL       string `yaml:"url"`
	SecretRef string `yaml:"secretRef"`
	Insecure  bool   `yaml:"insecure"`
}

// Backend represents a HAProxy backend configuration
type Backend struct {
	Name    string          `yaml:"name"`
	Mode    string          `yaml:"mode"`
	Balance BalanceConfig   `yaml:"balance"`
	Servers []ServerConfig  `yaml:"servers"`
}

// BalanceConfig represents load balancing configuration
type BalanceConfig struct {
	Algorithm string `yaml:"algorithm"`
}

// ServerConfig represents a backend server
type ServerConfig struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
	SSL     bool   `yaml:"ssl"`
	Check   bool   `yaml:"check"`
	Verify  string `yaml:"verify"`
}

// Frontend represents a HAProxy frontend configuration
type Frontend struct {
	Name              string             `yaml:"name"`
	Mode              string             `yaml:"mode"`
	Binds             []BindConfig       `yaml:"binds"`
	DefaultBackend    string             `yaml:"defaultBackend"`
	ACLs              []ACLConfig        `yaml:"acls,omitempty"`
	HTTPRequestRules  []HTTPRequestRule  `yaml:"httpRequestRules,omitempty"`
	HTTPResponseRules []HTTPResponseRule `yaml:"httpResponseRules,omitempty"`
	UseBackendRules   []UseBackendRule   `yaml:"useBackendRules,omitempty"`
}

// ACLConfig represents an ACL definition
type ACLConfig struct {
	Name      string `yaml:"name"`
	Criterion string `yaml:"criterion"`
	Value     string `yaml:"value"`
}

// HTTPRequestRule represents an HTTP request rule
type HTTPRequestRule struct {
	Index       *int   `yaml:"index,omitempty"`
	Type        string `yaml:"type"`
	Action      string `yaml:"action,omitempty"`
	DenyStatus  *int   `yaml:"denyStatus,omitempty"`
	Condition   string `yaml:"condition,omitempty"`
	CondTest    string `yaml:"condTest,omitempty"`
	HdrName     string `yaml:"hdrName,omitempty"`
	HdrFormat   string `yaml:"hdrFormat,omitempty"`
	PathMatch   string `yaml:"pathMatch,omitempty"`
	PathFmt     string `yaml:"pathFmt,omitempty"`
	RedirectType string `yaml:"redirectType,omitempty"`
	RedirectValue string `yaml:"redirectValue,omitempty"`
	RedirectCode *int   `yaml:"redirectCode,omitempty"`
	TrackKey    string `yaml:"trackKey,omitempty"`
	TrackTable  string `yaml:"trackTable,omitempty"`
	TrackStickCounter *int `yaml:"trackStickCounter,omitempty"`
}

// HTTPResponseRule represents an HTTP response rule
type HTTPResponseRule struct {
	Index     *int   `yaml:"index,omitempty"`
	Type      string `yaml:"type"`
	Action    string `yaml:"action,omitempty"`
	HdrName   string `yaml:"hdrName,omitempty"`
	HdrFormat string `yaml:"hdrFormat,omitempty"`
	Condition string `yaml:"condition,omitempty"`
	CondTest  string `yaml:"condTest,omitempty"`
}

// UseBackendRule represents a backend selection rule
type UseBackendRule struct {
	Index     *int   `yaml:"index,omitempty"`
	Name      string `yaml:"name"`
	Condition string `yaml:"condition,omitempty"`
	CondTest  string `yaml:"condTest,omitempty"`
}

// BindConfig represents a frontend bind configuration
type BindConfig struct {
	Name           string `yaml:"name"`
	Address        string `yaml:"address"`
	Port           int    `yaml:"port"`
	SSL            bool   `yaml:"ssl,omitempty"`
	SSLCertificate string `yaml:"sslCertificate,omitempty"`
	Alpn           string `yaml:"alpn,omitempty"`
	Verify         string `yaml:"verify,omitempty"`
}

// Client is a HAProxy Dataplane API client
type Client struct {
	config     *APIConfig
	httpClient *http.Client
	baseURL    *url.URL
}

// NewClient creates a new HAProxy client
func NewClient(config *APIConfig) *Client {
	baseURL, _ := url.Parse(config.URL)

	return &Client{
		config:  config,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ApplyConfiguration applies the configuration to HAProxy
func (c *Client) ApplyConfiguration(ctx context.Context, config *Config) error {
	// Apply backends
	for _, backend := range config.Backends {
		if err := c.applyBackend(ctx, &backend); err != nil {
			return fmt.Errorf("failed to apply backend %s: %w", backend.Name, err)
		}
	}

	// Apply frontends
	for _, frontend := range config.Frontends {
		if err := c.applyFrontend(ctx, &frontend); err != nil {
			return fmt.Errorf("failed to apply frontend %s: %w", frontend.Name, err)
		}
	}

	return nil
}

// applyBackend creates or updates a backend
func (c *Client) applyBackend(ctx context.Context, backend *Backend) error {
	// Check if backend exists
	exists, err := c.backendExists(ctx, backend.Name)
	if err != nil {
		return err
	}

	backendData := map[string]interface{}{
		"name":    backend.Name,
		"mode":    backend.Mode,
		"balance": map[string]interface{}{
			"algorithm": backend.Balance.Algorithm,
		},
	}

	if exists {
		// Update existing backend
		if err := c.doRequest(ctx, "PUT", fmt.Sprintf("/services/haproxy/configuration/backends/%s", backend.Name), backendData, nil); err != nil {
			return err
		}
	} else {
		// Create new backend
		if err := c.doRequest(ctx, "POST", "/services/haproxy/configuration/backends", backendData, nil); err != nil {
			return err
		}
	}

	// Apply servers
	for _, server := range backend.Servers {
		if err := c.applyServer(ctx, backend.Name, &server); err != nil {
			return fmt.Errorf("failed to apply server %s: %w", server.Name, err)
		}
	}

	return nil
}

// applyServer creates or updates a server in a backend
func (c *Client) applyServer(ctx context.Context, backendName string, server *ServerConfig) error {
	exists, err := c.serverExists(ctx, backendName, server.Name)
	if err != nil {
		return err
	}

	serverData := map[string]interface{}{
		"name":    server.Name,
		"address": server.Address,
		"port":    server.Port,
	}

	if server.SSL {
		serverData["ssl"] = "enabled"
		serverData["verify"] = server.Verify
	}

	if server.Check {
		serverData["check"] = "enabled"
	}

	endpoint := fmt.Sprintf("/services/haproxy/configuration/servers?backend=%s", backendName)

	if exists {
		// Update existing server
		if err := c.doRequest(ctx, "PUT", fmt.Sprintf("%s&name=%s", endpoint, server.Name), serverData, nil); err != nil {
			return err
		}
	} else {
		// Create new server
		if err := c.doRequest(ctx, "POST", endpoint, serverData, nil); err != nil {
			return err
		}
	}

	return nil
}

// applyFrontend creates or updates a frontend
func (c *Client) applyFrontend(ctx context.Context, frontend *Frontend) error {
	exists, err := c.frontendExists(ctx, frontend.Name)
	if err != nil {
		return err
	}

	frontendData := map[string]interface{}{
		"name":            frontend.Name,
		"mode":            frontend.Mode,
		"default_backend": frontend.DefaultBackend,
	}

	if exists {
		// Update existing frontend
		if err := c.doRequest(ctx, "PUT", fmt.Sprintf("/services/haproxy/configuration/frontends/%s", frontend.Name), frontendData, nil); err != nil {
			return err
		}
	} else {
		// Create new frontend
		if err := c.doRequest(ctx, "POST", "/services/haproxy/configuration/frontends", frontendData, nil); err != nil {
			return err
		}
	}

	// Apply binds
	for _, bind := range frontend.Binds {
		if err := c.applyBind(ctx, frontend.Name, &bind); err != nil {
			return fmt.Errorf("failed to apply bind %s: %w", bind.Name, err)
		}
	}

	// Apply ACLs
	for _, acl := range frontend.ACLs {
		if err := c.applyACL(ctx, frontend.Name, &acl); err != nil {
			return fmt.Errorf("failed to apply ACL %s: %w", acl.Name, err)
		}
	}

	// Apply HTTP request rules
	for i, rule := range frontend.HTTPRequestRules {
		if err := c.applyHTTPRequestRule(ctx, frontend.Name, i, &rule); err != nil {
			return fmt.Errorf("failed to apply HTTP request rule %d: %w", i, err)
		}
	}

	// Apply HTTP response rules
	for i, rule := range frontend.HTTPResponseRules {
		if err := c.applyHTTPResponseRule(ctx, frontend.Name, i, &rule); err != nil {
			return fmt.Errorf("failed to apply HTTP response rule %d: %w", i, err)
		}
	}

	// Apply backend switching rules
	for i, rule := range frontend.UseBackendRules {
		if err := c.applyBackendSwitchingRule(ctx, frontend.Name, i, &rule); err != nil {
			return fmt.Errorf("failed to apply backend switching rule %d: %w", i, err)
		}
	}

	return nil
}

// applyBind creates or updates a bind in a frontend
func (c *Client) applyBind(ctx context.Context, frontendName string, bind *BindConfig) error {
	exists, err := c.bindExists(ctx, frontendName, bind.Name)
	if err != nil {
		return err
	}

	bindData := map[string]interface{}{
		"name":    bind.Name,
		"address": bind.Address,
		"port":    bind.Port,
	}

	if bind.SSL {
		bindData["ssl"] = true
		if bind.SSLCertificate != "" {
			bindData["ssl_certificate"] = bind.SSLCertificate
		}
		if bind.Verify != "" {
			bindData["verify"] = bind.Verify
		}
	}

	if bind.Alpn != "" {
		bindData["alpn"] = bind.Alpn
	}

	endpoint := fmt.Sprintf("/services/haproxy/configuration/binds?frontend=%s", frontendName)

	if exists {
		// Update existing bind
		if err := c.doRequest(ctx, "PUT", fmt.Sprintf("%s&name=%s", endpoint, bind.Name), bindData, nil); err != nil {
			return err
		}
	} else {
		// Create new bind
		if err := c.doRequest(ctx, "POST", endpoint, bindData, nil); err != nil {
			return err
		}
	}

	return nil
}

// applyACL creates or updates an ACL in a frontend
func (c *Client) applyACL(ctx context.Context, frontendName string, acl *ACLConfig) error {
	exists, err := c.aclExists(ctx, frontendName, acl.Name)
	if err != nil {
		return err
	}

	aclData := map[string]interface{}{
		"acl_name": acl.Name,
		"criterion": acl.Criterion,
		"value":    acl.Value,
	}

	endpoint := fmt.Sprintf("/services/haproxy/configuration/acls?parent_type=frontend&parent_name=%s", frontendName)

	if exists {
		if err := c.doRequest(ctx, "PUT", fmt.Sprintf("%s&index=%s", endpoint, acl.Name), aclData, nil); err != nil {
			return err
		}
	} else {
		if err := c.doRequest(ctx, "POST", endpoint, aclData, nil); err != nil {
			return err
		}
	}

	return nil
}

// applyHTTPRequestRule creates or updates an HTTP request rule
func (c *Client) applyHTTPRequestRule(ctx context.Context, frontendName string, index int, rule *HTTPRequestRule) error {
	ruleData := map[string]interface{}{
		"index": index,
		"type":  rule.Type,
	}

	if rule.Action != "" {
		ruleData["action"] = rule.Action
	}
	if rule.DenyStatus != nil {
		ruleData["deny_status"] = *rule.DenyStatus
	}
	if rule.Condition != "" {
		ruleData["cond"] = rule.Condition
	}
	if rule.CondTest != "" {
		ruleData["cond_test"] = rule.CondTest
	}
	if rule.HdrName != "" {
		ruleData["hdr_name"] = rule.HdrName
	}
	if rule.HdrFormat != "" {
		ruleData["hdr_format"] = rule.HdrFormat
	}
	if rule.PathMatch != "" {
		ruleData["path_match"] = rule.PathMatch
	}
	if rule.PathFmt != "" {
		ruleData["path_fmt"] = rule.PathFmt
	}
	if rule.RedirectType != "" {
		ruleData["redir_type"] = rule.RedirectType
	}
	if rule.RedirectValue != "" {
		ruleData["redir_value"] = rule.RedirectValue
	}
	if rule.RedirectCode != nil {
		ruleData["redir_code"] = *rule.RedirectCode
	}
	if rule.TrackKey != "" {
		ruleData["track_key"] = rule.TrackKey
	}
	if rule.TrackTable != "" {
		ruleData["track_table"] = rule.TrackTable
	}
	if rule.TrackStickCounter != nil {
		ruleData["track_stick_counter"] = *rule.TrackStickCounter
	}

	endpoint := fmt.Sprintf("/services/haproxy/configuration/http_request_rules?parent_type=frontend&parent_name=%s&index=%d", frontendName, index)

	// Try update first, fall back to create
	if err := c.doRequest(ctx, "PUT", endpoint, ruleData, nil); err != nil {
		endpoint = fmt.Sprintf("/services/haproxy/configuration/http_request_rules?parent_type=frontend&parent_name=%s", frontendName)
		if err := c.doRequest(ctx, "POST", endpoint, ruleData, nil); err != nil {
			return err
		}
	}

	return nil
}

// applyHTTPResponseRule creates or updates an HTTP response rule
func (c *Client) applyHTTPResponseRule(ctx context.Context, frontendName string, index int, rule *HTTPResponseRule) error {
	ruleData := map[string]interface{}{
		"index": index,
		"type":  rule.Type,
	}

	if rule.Action != "" {
		ruleData["action"] = rule.Action
	}
	if rule.HdrName != "" {
		ruleData["hdr_name"] = rule.HdrName
	}
	if rule.HdrFormat != "" {
		ruleData["hdr_format"] = rule.HdrFormat
	}
	if rule.Condition != "" {
		ruleData["cond"] = rule.Condition
	}
	if rule.CondTest != "" {
		ruleData["cond_test"] = rule.CondTest
	}

	endpoint := fmt.Sprintf("/services/haproxy/configuration/http_response_rules?parent_type=frontend&parent_name=%s&index=%d", frontendName, index)

	if err := c.doRequest(ctx, "PUT", endpoint, ruleData, nil); err != nil {
		endpoint = fmt.Sprintf("/services/haproxy/configuration/http_response_rules?parent_type=frontend&parent_name=%s", frontendName)
		if err := c.doRequest(ctx, "POST", endpoint, ruleData, nil); err != nil {
			return err
		}
	}

	return nil
}

// applyBackendSwitchingRule creates or updates a backend switching rule
func (c *Client) applyBackendSwitchingRule(ctx context.Context, frontendName string, index int, rule *UseBackendRule) error {
	ruleData := map[string]interface{}{
		"index": index,
		"name":  rule.Name,
	}

	if rule.Condition != "" {
		ruleData["cond"] = rule.Condition
	}
	if rule.CondTest != "" {
		ruleData["cond_test"] = rule.CondTest
	}

	endpoint := fmt.Sprintf("/services/haproxy/configuration/backend_switching_rules?frontend=%s&index=%d", frontendName, index)

	if err := c.doRequest(ctx, "PUT", endpoint, ruleData, nil); err != nil {
		endpoint = fmt.Sprintf("/services/haproxy/configuration/backend_switching_rules?frontend=%s", frontendName)
		if err := c.doRequest(ctx, "POST", endpoint, ruleData, nil); err != nil {
			return err
		}
	}

	return nil
}

// backendExists checks if a backend exists
func (c *Client) backendExists(ctx context.Context, name string) (bool, error) {
	var result map[string]interface{}
	err := c.doRequest(ctx, "GET", fmt.Sprintf("/services/haproxy/configuration/backends/%s", name), nil, &result)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// serverExists checks if a server exists in a backend
func (c *Client) serverExists(ctx context.Context, backendName, serverName string) (bool, error) {
	var result map[string]interface{}
	endpoint := fmt.Sprintf("/services/haproxy/configuration/servers/%s?backend=%s", serverName, backendName)
	err := c.doRequest(ctx, "GET", endpoint, nil, &result)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// frontendExists checks if a frontend exists
func (c *Client) frontendExists(ctx context.Context, name string) (bool, error) {
	var result map[string]interface{}
	err := c.doRequest(ctx, "GET", fmt.Sprintf("/services/haproxy/configuration/frontends/%s", name), nil, &result)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// bindExists checks if a bind exists in a frontend
func (c *Client) bindExists(ctx context.Context, frontendName, bindName string) (bool, error) {
	var result map[string]interface{}
	endpoint := fmt.Sprintf("/services/haproxy/configuration/binds/%s?frontend=%s", bindName, frontendName)
	err := c.doRequest(ctx, "GET", endpoint, nil, &result)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// aclExists checks if an ACL exists in a frontend
func (c *Client) aclExists(ctx context.Context, frontendName, aclName string) (bool, error) {
	var result map[string]interface{}
	endpoint := fmt.Sprintf("/services/haproxy/configuration/acls/%s?parent_type=frontend&parent_name=%s", aclName, frontendName)
	err := c.doRequest(ctx, "GET", endpoint, nil, &result)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// doRequest performs an HTTP request to the HAProxy API
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	url := c.baseURL.ResolveReference(&url.URL{Path: path}).String()

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.config.Username, c.config.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
		}
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// APIError represents an API error
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}

// isNotFoundError checks if an error is a 404 Not Found
func isNotFoundError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// ParseConfig parses YAML configuration
func ParseConfig(data []byte) (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return &config, nil
}

// HashConfig creates a hash of the configuration
func HashConfig(data string) string {
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}
