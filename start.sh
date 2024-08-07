#!/bin/sh

# Print directory contents
echo "Directory contents:"
ls -la

# Check if .air.toml exists
if [ -f ".air.toml" ]; then
    echo ".air.toml found, starting with Air"
    air -c .air.toml
else
    echo ".air.toml not found, starting Go application directly"
    go run main.go
fi
