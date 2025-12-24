#!/usr/bin/env bash
set -eu

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Path to pull-secret.txt in the same directory as this script
PULL_SECRET="${SCRIPT_DIR}/pull-secret.txt"

# Check if pull-secret.txt exists
if [ ! -f "$PULL_SECRET" ]; then
    echo "Error: pull-secret.txt not found at ${PULL_SECRET}"
    echo "Please place pull-secret.txt in the same directory as this script (${SCRIPT_DIR})"
    exit 1
fi

# Run CRC setup and handle anonymous stats prompt
echo "Running CRC setup..."
echo "Setting CRC preset to 'openshift'..."
echo "n" | crc setup

echo "Enabling cluster monitoring..."
crc config set enable-cluster-monitoring true

echo "Starting CRC (initial)..."
crc start -p "$PULL_SECRET"

echo "Stopping CRC to reconfigure..."
crc stop

echo "Setting memory to 32GB, DiskSize to 90GB, CPUs to 8"
crc config set cpus 8
crc config set memory 32768
crc config set disk-size 90

# Output files in script directory
CRC_OUTPUT="${SCRIPT_DIR}/crc-output.txt"
CRC_CREDENTIALS="${SCRIPT_DIR}/crc-credentials.txt"

# Start CRC again and capture output
echo "Starting CRC with new configuration..."
crc start -p "$PULL_SECRET" | tee "$CRC_OUTPUT"

# Extract credentials
OC_ADMIN_USER=$(grep -A 1 "Log in as administrator:" "$CRC_OUTPUT" | grep "Username:" | awk '{print $2}')
OC_ADMIN_PASS=$(grep -A 2 "Log in as administrator:" "$CRC_OUTPUT" | grep "Password:" | awk '{print $2}')
OC_DEV_USER=$(grep -A 1 "Log in as user:" "$CRC_OUTPUT" | grep "Username:" | awk '{print $2}')
OC_DEV_PASS=$(grep -A 2 "Log in as user:" "$CRC_OUTPUT" | grep "Password:" | awk '{print $2}')

export OC_ADMIN_PASS=$OC_ADMIN_PASS
export OC_ADMIN_USER=$OC_ADMIN_USER
export OC_DEV_USER=$OC_DEV_USER
export OC_DEV_PASS=$OC_DEV_PASS

# check if crc-credentials.txt exists, if so, backupfile and create new one
if [ -f "$CRC_CREDENTIALS" ]; then
    echo "Backing up existing crc-credentials.txt to crc-credentials.txt.bak$(date +%Y%m%d%H%M%S)"
    cp "$CRC_CREDENTIALS" "${CRC_CREDENTIALS}.bak$(date +%Y%m%d%H%M%S)"
fi

# Save credentials
cat > "$CRC_CREDENTIALS" <<EOF
OC_ADMIN_USER: $OC_ADMIN_USER
OC_ADMIN_PASS: $OC_ADMIN_PASS
OC_DEV_USER: $OC_DEV_USER
OC_DEV_PASS: $OC_DEV_PASS
EOF

echo ""
echo "Setup complete! Credentials saved to ${CRC_CREDENTIALS}"
echo "CRC output saved to ${CRC_OUTPUT}"
echo "exported as env variables: OC_ADMIN_USER, OC_ADMIN_PASS, OC_DEV_USER, OC_DEV_PASS"
