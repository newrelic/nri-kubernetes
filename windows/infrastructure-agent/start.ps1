Write-Output 'Starting up...'
Start-Sleep 10
Write-Output "Script is about to start newrelic-infra.exe"

# Starts the Infra Agent as the main process
& "C:\Program Files\New Relic\newrelic-infra\newrelic-infra.exe" | Out-Default

# If the script gets to here, the agent has stopped
Write-Output 'The Infra Agent has stopped'
