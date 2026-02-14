Write-Output 'Starting up...'
Start-Sleep 10
Write-Output "Script is about to start newrelic-infra.exe"

# Determine the correct path based on whether we're in hostProcess mode
$agentPath = "C:\Program Files\New Relic\newrelic-infra\newrelic-infra.exe"
if ($env:CONTAINER_SANDBOX_MOUNT_POINT) {
    # In hostProcess mode, reference the container filesystem
    $agentPath = "$env:CONTAINER_SANDBOX_MOUNT_POINT\Program Files\New Relic\newrelic-infra\newrelic-infra.exe"
    Write-Output "Running in hostProcess mode, using path: $agentPath"
}

# Starts the Infra Agent as the main process
& $agentPath | Out-Default

# If the script gets to here, the agent has stopped
Write-Output 'The Infra Agent has stopped'
