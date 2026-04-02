param(
    [Parameter(Mandatory = $true)]
    [string]$JsonArgs
)

$ErrorActionPreference = "Stop"

function Send-Response {
    param($Text, $IsError = $false)
    $response = @{
        content = @(@{ type = "text"; text = $Text })
    }
    if ($IsError) { $response.isError = $true }
    $response | ConvertTo-Json -Compress -Depth 5 | Write-Host
    if ($IsError) { exit 1 } else { exit 0 }
}

try { $args_obj = $JsonArgs | ConvertFrom-Json } catch { Send-Response "Failed to parse JSON: $_" $true }

$check       = $args_obj.check
$target      = $args_obj.target
$port        = $args_obj.port
$path        = if ($args_obj.path) { $args_obj.path } else { "/" }
$scheme      = if ($args_obj.scheme) { $args_obj.scheme } else { "http" }
$timeout_sec = if ($args_obj.timeout) { [int]$args_obj.timeout } else { 5 }

if (-not $target) { Send-Response "Error: 'target' parameter is required." $true }

switch ($check) {
    "ping" {
        try {
            $result = Test-Connection -ComputerName $target -Count 3 -ErrorAction Stop
            $summary = $result | ForEach-Object {
                "Reply from $($_.Address): time=$($_.ResponseTime)ms"
            }
            Send-Response "HEALTHY: $target is reachable.`n`n$($summary -join "`n")"
        }
        catch {
            Send-Response "UNHEALTHY: $target is not reachable. $_" $true
        }
    }
    "port" {
        if (-not $port) { Send-Response "Error: 'port' parameter is required for port check." $true }
        try {
            $tcp = New-Object System.Net.Sockets.TcpClient
            $connect = $tcp.BeginConnect($target, $port, $null, $null)
            $wait = $connect.AsyncWaitHandle.WaitOne($timeout_sec * 1000, $false)
            if ($wait -and $tcp.Connected) {
                $tcp.EndConnect($connect)
                $tcp.Close()
                Send-Response "HEALTHY: ${target}:${port} is open and accepting connections."
            }
            else {
                $tcp.Close()
                Send-Response "UNHEALTHY: ${target}:${port} is not reachable or refused connection." $true
            }
        }
        catch {
            Send-Response "UNHEALTHY: ${target}:${port} connection failed: $_" $true
        }
    }
    "http" {
        $url = "${scheme}://${target}"
        if ($port) { $url = "${scheme}://${target}:${port}" }
        $url = "${url}${path}"

        try {
            # Disable SSL errors for self-signed certs common in homelabs
            [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }

            $web = [System.Net.HttpWebRequest]::Create($url)
            $web.Timeout = $timeout_sec * 1000
            $web.Method = "GET"

            $resp = $web.GetResponse()
            $status_code = [int]$resp.StatusCode
            $reader = New-Object System.IO.StreamReader($resp.GetResponseStream())
            $body = $reader.ReadToEnd()
            $reader.Close()
            $resp.Close()

            if ($body.Length -gt 2048) { $body = $body.Substring(0, 2048) }
            Send-Response "HEALTHY: $url returned HTTP $status_code.`n`nResponse (truncated):`n$body"
        }
        catch [System.Net.WebException] {
            $status = ""
            if ($_.Exception.Response) {
                $status = " (HTTP $([int]$_.Exception.Response.StatusCode))"
            }
            Send-Response "UNHEALTHY: $url is not responding${status}. $($_.Exception.Message)" $true
        }
        catch {
            Send-Response "UNHEALTHY: $url connection failed: $_" $true
        }
    }
    default {
        Send-Response "Unknown check type: '$check'. Valid types: ping, port, http" $true
    }
}
