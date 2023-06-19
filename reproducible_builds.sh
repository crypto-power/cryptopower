#!/bin/bash

# Copy files from reproducible-builds to cryptopower
rsync -av --exclude='README.md' reproducible-builds/ .

# Execute make command
sudo make linux-amd64-binary