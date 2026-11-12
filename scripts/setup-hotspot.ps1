# Run from an Administrator Windows PowerShell terminal.
# Optional: -HotspotIP 192.168.137.1 -Distro Ubuntu
[CmdletBinding()]
param(
    [string]$HotspotIP,
    [string]$Distro
)
$ErrorActionPreference = 'Stop'
$Port = 45873
$RuleName = 'RummyHotspot45873'
$Identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$Principal = New-Object Security.Principal.WindowsPrincipal($Identity)
if (-not $Principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Open Windows PowerShell as Administrator and run this script again.'
}

if (-not $HotspotIP) {
    $Adapters = @(Get-NetAdapter -IncludeHidden | Where-Object {
        $_.Status -eq 'Up' -and $_.InterfaceDescription -like '*Wi-Fi Direct*'
    })
    $Candidates = @(foreach ($Adapter in $Adapters) {
        Get-NetIPAddress -InterfaceIndex $Adapter.ifIndex -AddressFamily IPv4 |
            Where-Object { $_.IPAddress -notlike '169.254.*' }
    })
    if ($Candidates.Count -eq 1) {
        $HotspotIP = $Candidates[0].IPAddress
    } else {
        Get-NetIPAddress -AddressFamily IPv4 |
            Select-Object InterfaceAlias, IPAddress | Format-Table | Out-Host
        $HotspotIP = Read-Host 'Enter the IPv4 address of your Windows Mobile hotspot adapter'
    }
}
$ParsedIP = $null
if (-not [Net.IPAddress]::TryParse($HotspotIP, [ref]$ParsedIP) -or
    $ParsedIP.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork -or
    -not (Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.IPAddress -eq $HotspotIP })) {
    throw 'That IPv4 address is not assigned to a Windows adapter. Turn on Mobile hotspot and try again.'
}

$WslArgs = @()
if ($Distro) { $WslArgs += @('--distribution', $Distro) }
$WslArgs += @('--', 'hostname', '-I')
$AddressOutput = & wsl.exe @WslArgs
if ($LASTEXITCODE -ne 0) { throw 'Could not read the WSL address. Check the distro name.' }
$WslIP = (($AddressOutput -join ' ').Trim() -split '\s+')[0]
$ParsedWslIP = $null
if (-not [Net.IPAddress]::TryParse($WslIP, [ref]$ParsedWslIP) -or
    $ParsedWslIP.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork) {
    throw 'Could not determine a WSL IPv4 address.'
}

Start-Service iphlpsvc
& netsh.exe interface portproxy add v4tov4 "listenaddress=$HotspotIP" "listenport=$Port" "connectaddress=$WslIP" "connectport=$Port"
if ($LASTEXITCODE -ne 0) { throw 'Failed to add the Windows-to-WSL forwarding rule.' }

if (Get-NetFirewallRule -Name $RuleName -ErrorAction SilentlyContinue) {
    Set-NetFirewallRule -Name $RuleName -Enabled True -Direction Inbound -Action Allow -Profile Any
    Get-NetFirewallRule -Name $RuleName | Get-NetFirewallAddressFilter |
        Set-NetFirewallAddressFilter -LocalAddress $HotspotIP -RemoteAddress LocalSubnet
    Get-NetFirewallRule -Name $RuleName | Get-NetFirewallPortFilter |
        Set-NetFirewallPortFilter -Protocol TCP -LocalPort $Port
} else {
    New-NetFirewallRule -Name $RuleName -DisplayName 'Rummy hotspot TCP 45873' `
        -Direction Inbound -Action Allow -Protocol TCP -LocalAddress $HotspotIP `
        -LocalPort $Port -RemoteAddress LocalSubnet -Profile Any | Out-Null
}

Write-Host "Configured $HotspotIP`:$Port -> $WslIP`:$Port"
Write-Host "Enter $HotspotIP in Rummy's Host address screen. Share the resulting full join code."
Write-Host 'Rerun this script if WSL restarts and its IP changes.'
Write-Host 'With the game hosting, your friend can check the port from Windows PowerShell:'
Write-Host "Test-NetConnection $HotspotIP -Port $Port"
Write-Host 'This configures forwarding and the Windows rule; guest connectivity still needs checking.'
