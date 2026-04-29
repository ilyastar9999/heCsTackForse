param()

$action = $env:HECSTACK_VPN_ACTION
$teamId = $env:HECSTACK_VPN_TEAM_ID
$publicKey = $env:HECSTACK_VPN_PUBLIC_KEY
$allowedSubnet = $env:HECSTACK_VPN_ALLOWED_SUBNET

if ($action -ne 'provision') {
  Write-Error "unsupported action: $action"
  exit 1
}

# Replace this with your actual gateway logic, for example:
# ssh wg-gateway "wg set wg0 peer $publicKey allowed-ips $allowedSubnet"

Write-Output "provisioned team $teamId peer $publicKey ($allowedSubnet)"
