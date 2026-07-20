package deployer

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type PVEDeployer struct {
	URL         string
	TokenID     string
	TokenSecret string
	Node        string
	VMTemplate  int
	client      *http.Client
}

func NewPVEDeployer(url, tokenID, tokenSecret, node string, vmTemplate int, insecureSkipVerify bool) *PVEDeployer {
	return &PVEDeployer{
		URL:         url,
		TokenID:     tokenID,
		TokenSecret: tokenSecret,
		Node:        node,
		VMTemplate:  vmTemplate,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// InsecureSkipVerify is user-controlled via config (insecure_skip_verify).
				// Only enable when the PVE server uses a self-signed certificate.
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec
			},
		},
	}
}

func (p *PVEDeployer) Type() string { return "pve" }

func (p *PVEDeployer) doRequest(ctx context.Context, method, path string, body string) ([]byte, error) {
	url := p.URL + path
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", p.TokenID, p.TokenSecret))
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pve api returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (p *PVEDeployer) Deploy(ctx context.Context, req DeployRequest) (*models.Instance, error) {
	vmTemplate := req.VMTemplate
	if vmTemplate == 0 {
		vmTemplate = p.VMTemplate
	}
	newVMID := fmt.Sprintf("%d", time.Now().UnixNano()%9000+1000)
	body := fmt.Sprintf("newid=%s&name=ctf-%d&full=1", newVMID, req.ChallengeID)
	_, err := p.doRequest(ctx, "POST", fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/clone", p.Node, vmTemplate), body)
	if err != nil {
		return nil, fmt.Errorf("pve clone failed: %w", err)
	}
	inst := &models.Instance{
		ChallengeID:    req.ChallengeID,
		UserID:         req.UserID,
		TeamID:         req.TeamID,
		InstanceType:   req.DeployType,
		Backend:        "pve",
		InstanceID:     newVMID,
		ConnectionInfo: fmt.Sprintf(`{"vmid":"%s","node":"%s"}`, newVMID, p.Node),
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	return inst, nil
}

func (p *PVEDeployer) Destroy(ctx context.Context, instanceID string) error {
	_, err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/api2/json/nodes/%s/qemu/%s", p.Node, instanceID), "")
	if err != nil {
		return fmt.Errorf("pve destroy failed: %w", err)
	}
	return nil
}

func (p *PVEDeployer) Status(ctx context.Context, instanceID string) (InstanceStatus, error) {
	out, err := p.doRequest(ctx, "GET", fmt.Sprintf("/api2/json/nodes/%s/qemu/%s/status/current", p.Node, instanceID), "")
	if err != nil {
		return StatusError, nil
	}
	s := string(out)
	if strings.Contains(s, `"running"`) {
		return StatusRunning, nil
	}
	if strings.Contains(s, `"stopped"`) {
		return StatusStopped, nil
	}
	return StatusPending, nil
}
