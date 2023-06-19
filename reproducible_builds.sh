#!/bin/bash

# Copy files from reproducible-builds to cryptopower
rsync -av --exclude='README.md' reproduciblebuilds/ .

# Execute make command
sudo make "$1"