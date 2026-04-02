if ($args -contains "--mcp-metadata") {
    $metadata = @{
        name = "test_resource"
        description = "Returns a test resource."
        inputSchema = @{
            type = "object"
            properties = @{}
        }
    }
    Write-Output ($metadata | ConvertTo-Json -Compress)
    exit 0
}

$resource = @{
    type = "resource"
    resource = @{
        uri = "test://resource"
        mimeType = "text/plain"
        text = "This is a test resource."
    }
}

$resource | ConvertTo-Json -Compress
