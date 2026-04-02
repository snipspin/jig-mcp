if ($args -contains "--mcp-metadata") {
    $metadata = @{
        name = "test_invalid_image"
        description = "Returns an invalid base64 image."
        inputSchema = @{
            type = "object"
            properties = @{}
        }
    }
    Write-Output ($metadata | ConvertTo-Json -Compress)
    exit 0
}

# Invalid base64 (contains spaces or just wrong)
$invalidData = "not-base64-!!!"

$result = @{
    type = "image"
    mimeType = "image/png"
    data = $invalidData
}

$result | ConvertTo-Json -Compress
