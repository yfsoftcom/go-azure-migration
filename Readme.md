# Go Azure Migration

Here is a command tool for migrating massive blobs from Azure account to another Azure account.

Steps:
- list source blobs
- list destination blobs
- calculate diff
- azCopy

Required env:
- AZURE_SOURCE_KEY
- AZURE_DIST_KEY

Required exec


Run command:
./main -worker=5 -total=100000 -max=480000 -min=470000 -folder=true