if ($args -contains "--mcp-metadata") {
    $metadata = @{
        name = "test_image"
        description = "Returns a test red dot PNG image."
        inputSchema = @{
            type = "object"
            properties = @{}
        }
    }
    Write-Output ($metadata | ConvertTo-Json -Compress)
    exit 0
}

# The 1x1 red dot base64
$redDot = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="

# The requirement says "If a script returns {"type":"image",...}, pass it through correctly".
$result = @{
    type = "image"
    mimeType = "image/png"
    data = $redDot
}

$result | ConvertTo-Json -Compress
