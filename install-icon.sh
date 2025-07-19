#!/bin/bash
# Script to install TotalRecall icon for GNOME on Fedora Linux
# Run with sudo for system-wide installation

set -e

# Check if running as root for system-wide install
if [[ $EUID -eq 0 ]]; then
   echo "Installing TotalRecall icon system-wide..."
   ICON_BASE="/usr/share/icons/hicolor"
   APP_DIR="/usr/share/applications"
   BINARY_PATH="/usr/local/bin/totalrecall"
else
   echo "Installing TotalRecall icon for current user..."
   ICON_BASE="$HOME/.local/share/icons/hicolor"
   APP_DIR="$HOME/.local/share/applications"
   BINARY_PATH="$HOME/go/bin/totalrecall"
fi

# Create directories
mkdir -p "$ICON_BASE"/{16x16,32x32,48x48,64x64,128x128,256x256,512x512}/apps
mkdir -p "$APP_DIR"

# Copy icons
cp assets/icons/totalrecall_16.png "$ICON_BASE/16x16/apps/totalrecall.png"
cp assets/icons/totalrecall_32.png "$ICON_BASE/32x32/apps/totalrecall.png"
cp assets/icons/totalrecall_48.png "$ICON_BASE/48x48/apps/totalrecall.png"
cp assets/icons/totalrecall_64.png "$ICON_BASE/64x64/apps/totalrecall.png"
cp assets/icons/totalrecall_128.png "$ICON_BASE/128x128/apps/totalrecall.png"
cp assets/icons/totalrecall_256.png "$ICON_BASE/256x256/apps/totalrecall.png"
cp assets/icons/totalrecall_512.png "$ICON_BASE/512x512/apps/totalrecall.png"

# Copy scalable icon
mkdir -p "$ICON_BASE/scalable/apps"
cp assets/icons/totalrecall.svg "$ICON_BASE/scalable/apps/totalrecall.svg"

# Copy and update desktop file with correct binary path
# First check if totalrecall is in standard locations
if command -v totalrecall &> /dev/null; then
    TOTALRECALL_PATH=$(command -v totalrecall)
elif [[ -x "$HOME/go/bin/totalrecall" ]]; then
    TOTALRECALL_PATH="$HOME/go/bin/totalrecall"
elif [[ -x "/usr/local/bin/totalrecall" ]]; then
    TOTALRECALL_PATH="/usr/local/bin/totalrecall"
elif [[ -x "/usr/bin/totalrecall" ]]; then
    TOTALRECALL_PATH="/usr/bin/totalrecall"
else
    echo "Warning: totalrecall binary not found in standard locations"
    echo "Please install totalrecall first with 'task install' or 'go install ./cmd/totalrecall'"
    TOTALRECALL_PATH="totalrecall"
fi

# Create desktop file with proper exec path
sed "s|Exec=totalrecall|Exec=$TOTALRECALL_PATH|g" totalrecall.desktop > "$APP_DIR/totalrecall.desktop"

# Update caches
if command -v gtk-update-icon-cache &> /dev/null; then
    gtk-update-icon-cache -f -t "$ICON_BASE" 2>/dev/null || true
fi

if command -v update-desktop-database &> /dev/null; then
    update-desktop-database "$APP_DIR" 2>/dev/null || true
fi

echo "TotalRecall icon installed successfully!"
echo "You may need to log out and log back in to see the icon in GNOME."