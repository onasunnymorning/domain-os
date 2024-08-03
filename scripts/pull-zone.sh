#!/bin/sh

# This script pulls the TLD records, DomainDelegations and Glue records for a zone
# It uses the following parameters:
# $1 - the zone name
# $2 - the output file name (output dir is ./)
# $3 - the hostname of the server that is running the DOS API

ZONE=$1
FILENAME=$2
HOSTNAME=$3

if [ -z "$ZONE" ]; then
  echo "Please provide the zone name"
  exit 1
fi

if [ -z "$FILENAME" ]; then
  echo "Please provide the output file name"
  exit 1
fi

if [ -z "$HOSTNAME" ]; then
  echo "Please provide the hostname of the server to query using dig"
  exit 1
fi

# Logic
# It will pull the respective records in text format form the API and save them in the output file
# It will use named-checkzone to validate the zone
# It will print out the zone

# After that
# reload/reconfig the zone/server

# Pull the TLD records
curl --location http://$HOSTNAME:8080/tlds/$ZONE/dns/resource-records?format=text > $FILENAME

# Pull the DomainDelegations
curl --location http://$HOSTNAME:8080/dns/$ZONE/domains/delegations?format=text >> $FILENAME

# Validate the zone
named-checkzone $ZONE $FILENAME

# Print out the zone
cat $FILENAME
