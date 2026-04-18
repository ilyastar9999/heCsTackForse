package ad

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// GenerateWireGuardKeys creates a new WireGuard-compatible Curve25519 keypair.
// Both keys are returned as standard Base64 strings (same encoding used by `wg`).
func GenerateWireGuardKeys() (privateKeyB64, publicKeyB64 string, err error) {
	var privKey [32]byte
	if _, err = rand.Read(privKey[:]); err != nil {
		return
	}
	// Apply WireGuard key clamping as per RFC 7748 §5
	privKey[0] &= 248
	privKey[31] = (privKey[31] & 127) | 64

	pubKey, e := curve25519.X25519(privKey[:], curve25519.Basepoint)
	if e != nil {
		err = e
		return
	}
	privateKeyB64 = base64.StdEncoding.EncodeToString(privKey[:])
	publicKeyB64 = base64.StdEncoding.EncodeToString(pubKey)
	return
}

// BuildClientConfig returns the text of a WireGuard client .conf file for the
// given team.  serverPublicKey, serverEndpoint, serverIP, teamIP, gameNetCIDR
// and dns are all taken from the platform config / database.
func BuildClientConfig(
	teamPrivateKey string,
	teamIP string, // e.g. "10.8.1.1/24"
	serverPublicKey string,
	serverEndpoint string, // host:port
	serverIP string, // e.g. "10.8.0.1"
	gameNetCIDR string, // e.g. "10.10.0.0/16"
	dns string,
) string {
	// AllowedIPs routes:  the game network + the WireGuard server IP
	allowedIPs := gameNetCIDR
	if serverIP != "" {
		allowedIPs = fmt.Sprintf("%s, %s/32", allowedIPs, serverIP)
	}

	dnsLine := ""
	if dns != "" {
		dnsLine = fmt.Sprintf("DNS = %s\n", dns)
	}

	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
%s
[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, teamPrivateKey, teamIP, dnsLine, serverPublicKey, serverEndpoint, allowedIPs)
}
