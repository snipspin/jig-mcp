# hello.ps1 - Example jig-mcp tool script (PowerShell)
# Reads JSON arguments and outputs a greeting

param(
    [string]$ArgsJson
)

# Parse the JSON argument
$ArgsObj = $ArgsJson | ConvertFrom-Json

# Extract the name parameter (default to "World" if not provided)
$Name = if ($ArgsObj.name) { $ArgsObj.name } else { "World" }

# Output the greeting as a JSON object (MCP-compliant format)
$Response = @{
    content = @(
        @{
            type = "text"
            text = "Hello, $Name! This is an example jig-mcp tool."
        }
    )
}

$Response | ConvertTo-Json -Depth 3
