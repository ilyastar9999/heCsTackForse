package ad

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/config"
	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func RunVPNHook(cfg config.VPNConfig, action string, peer models.VPNPeer, clientAddress string) error {
	if strings.TrimSpace(cfg.HookCommand) == "" {
		return nil
	}

	timeout := 15 * time.Second
	if strings.TrimSpace(cfg.HookTimeout) != "" {
		parsed, err := time.ParseDuration(cfg.HookTimeout)
		if err != nil {
			return fmt.Errorf("invalid vpn hook timeout: %w", err)
		}
		timeout = parsed
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cfg.HookCommand, cfg.HookArgs...)
	cmd.Env = append(os.Environ(),
		"HECSTACK_VPN_ACTION="+strings.TrimSpace(action),
		"HECSTACK_VPN_TEAM_ID="+strconv.FormatInt(peer.TeamID, 10),
		"HECSTACK_VPN_PEER_ID="+strconv.FormatInt(peer.ID, 10),
		"HECSTACK_VPN_PRIVATE_KEY="+peer.PrivateKey,
		"HECSTACK_VPN_PUBLIC_KEY="+peer.PublicKey,
		"HECSTACK_VPN_ALLOWED_SUBNET="+peer.AllowedIP,
		"HECSTACK_VPN_CLIENT_ADDRESS="+clientAddress,
		"HECSTACK_VPN_SERVER_PUBLIC_KEY="+cfg.ServerPublicKey,
		"HECSTACK_VPN_SERVER_ENDPOINT="+cfg.ServerEndpoint,
		"HECSTACK_VPN_SERVER_IP="+cfg.ServerIP,
		"HECSTACK_VPN_GAME_NET_CIDR="+cfg.GameNetCIDR,
		"HECSTACK_VPN_DNS="+cfg.DNS,
	)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("vpn hook timed out after %s", timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("vpn hook failed: %s", msg)
	}
	return nil
}
