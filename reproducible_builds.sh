#!/bin/bash

# Copy files from reproducible-builds to cryptopower
rsync -av --exclude='README.md' --exclude='cryptopower*' reproduciblebuilds/ .

# Execute make command
sudo make "$1"