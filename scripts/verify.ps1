param(
    [int]$OrderPort = 18080,
    [int]$InventoryPort = 18081,
    [string]$GoExe = 'go'
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $projectRoot 'bin'
$orderProcess = $null
$inventoryProcess = $null
$savedOrderAddr = $env:ORDER_ADDR
$savedInventoryAddr = $env:INVENTORY_ADDR
$savedInventoryURL = $env:INVENTORY_URL
$savedGoCache = $env:GOCACHE
$savedGoModCache = $env:GOMODCACHE

function Wait-Service($Process, [string]$URL, [string]$ExpectedStatus) {
    for ($attempt = 0; $attempt -lt 50; $attempt++) {
        if ($Process.HasExited) {
            throw "Service exited during startup. See logs in $binDir."
        }
        $code = & curl.exe --silent --output NUL --max-time 1 --write-out '%{http_code}' $URL
        if ($code -eq $ExpectedStatus) { return }
        Start-Sleep -Milliseconds 100
    }
    throw "Service did not become ready at $URL"
}

function Show-Request([string]$Label, [string]$Method, [string]$URL, [int]$ExpectedStatus, [string]$Body = '') {
    Write-Output "`n$Label"
    Write-Output "$Method $URL"
    $curlArgs = @('--silent', '--show-error', '--max-time', '5', '--write-out', "`nHTTP %{http_code}", '--request', $Method)
    if ($Body) {
        $requestPath = Join-Path $binDir 'request.json'
        [System.IO.File]::WriteAllText($requestPath, $Body, [System.Text.UTF8Encoding]::new($false))
        $curlArgs += @('--header', 'Content-Type: application/json', '--data-binary', "@$requestPath")
        Write-Output $Body
    }
    $output = & curl.exe @curlArgs $URL
    if ($LASTEXITCODE -ne 0) { throw "curl failed for $URL" }
    $output | Write-Output
    if (($output -join "`n") -notmatch "HTTP $ExpectedStatus$") {
        throw "Expected HTTP $ExpectedStatus"
    }
}

Push-Location $projectRoot
try {
    Get-Command curl.exe -ErrorAction Stop | Out-Null
    if (-not (Get-Command $GoExe -ErrorAction SilentlyContinue)) {
        $GoExe = 'C:\Program Files\Go\bin\go.exe'
        if (-not (Test-Path -LiteralPath $GoExe)) { throw 'Install Go and add it to PATH, or pass -GoExe.' }
    }
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    $env:GOCACHE = Join-Path $projectRoot '.cache\go-build'
    $env:GOMODCACHE = Join-Path $projectRoot '.cache\go-mod'
    & $GoExe build -o (Join-Path $binDir 'inventory.exe') ./cmd/inventory
    if ($LASTEXITCODE -ne 0) { throw 'Inventory build failed' }
    & $GoExe build -o (Join-Path $binDir 'order.exe') ./cmd/order
    if ($LASTEXITCODE -ne 0) { throw 'Order build failed' }

    $env:ORDER_ADDR = "127.0.0.1:$OrderPort"
    $env:INVENTORY_ADDR = "127.0.0.1:$InventoryPort"
    $env:INVENTORY_URL = "http://127.0.0.1:$InventoryPort"
    $inventoryProcess = Start-Process -FilePath (Join-Path $binDir 'inventory.exe') -PassThru -WindowStyle Hidden -RedirectStandardOutput (Join-Path $binDir 'inventory.stdout.log') -RedirectStandardError (Join-Path $binDir 'inventory.stderr.log')
    Wait-Service $inventoryProcess "$env:INVENTORY_URL/inventory/availability?item_id=item-1&quantity=1" '200'
    $orderProcess = Start-Process -FilePath (Join-Path $binDir 'order.exe') -PassThru -WindowStyle Hidden -RedirectStandardOutput (Join-Path $binDir 'order.stdout.log') -RedirectStandardError (Join-Path $binDir 'order.stderr.log')
    $orderURL = "http://127.0.0.1:$OrderPort"
    Wait-Service $orderProcess "$orderURL/orders/missing" '404'
    Write-Output "Independent processes: Order PID=$($orderProcess.Id), Inventory PID=$($inventoryProcess.Id)"

    Show-Request 'Inventory query' 'GET' "$env:INVENTORY_URL/inventory/availability?item_id=item-1&quantity=2" 200
    Show-Request 'Successful order' 'POST' "$orderURL/orders" 201 '{"item_id":"item-1","quantity":2}'
    Show-Request 'Unavailable item' 'POST' "$orderURL/orders" 409 '{"item_id":"item-2","quantity":1}'
    Show-Request 'Invalid quantity' 'POST' "$orderURL/orders" 400 '{"item_id":"item-1","quantity":0}'

    Stop-Process -Id $inventoryProcess.Id
    $inventoryProcess.WaitForExit()
    Write-Output "`nStopped Inventory PID=$($inventoryProcess.Id)"
    Show-Request 'Dependency failure' 'POST' "$orderURL/orders" 503 '{"item_id":"item-1","quantity":2}'
    Show-Request 'Repeated dependency failure' 'POST' "$orderURL/orders" 503 '{"item_id":"item-1","quantity":2}'
    Show-Request 'Stored order still accessible during outage' 'GET' "$orderURL/orders/order-1" 200
    if ($orderProcess.HasExited) { throw 'Order Service exited during the outage' }
    Write-Output "Order PID=$($orderProcess.Id) remains running."

    $inventoryProcess = Start-Process -FilePath (Join-Path $binDir 'inventory.exe') -PassThru -WindowStyle Hidden -RedirectStandardOutput (Join-Path $binDir 'inventory-restarted.stdout.log') -RedirectStandardError (Join-Path $binDir 'inventory-restarted.stderr.log')
    Wait-Service $inventoryProcess "$env:INVENTORY_URL/inventory/availability?item_id=item-1&quantity=1" '200'
    Write-Output "`nRestarted Inventory PID=$($inventoryProcess.Id)"
    Show-Request 'Recovery with the same Order process' 'POST' "$orderURL/orders" 201 '{"item_id":"item-1","quantity":2}'
    Write-Output "`nPASS: success, rejection, dependency failure, continued operation, and recovery verified."
}
finally {
    foreach ($serviceProcess in @($orderProcess, $inventoryProcess)) {
        if ($null -ne $serviceProcess -and -not $serviceProcess.HasExited) {
            Stop-Process -Id $serviceProcess.Id -ErrorAction SilentlyContinue
            $serviceProcess.WaitForExit()
        }
    }
    $env:ORDER_ADDR = $savedOrderAddr
    $env:INVENTORY_ADDR = $savedInventoryAddr
    $env:INVENTORY_URL = $savedInventoryURL
    $env:GOCACHE = $savedGoCache
    $env:GOMODCACHE = $savedGoModCache
    Pop-Location
}
