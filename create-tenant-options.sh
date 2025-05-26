#!/bin/sh

c8y tenantoptions create --category c8y-devmgmt-repo-intgr --key fwStorageProvider --value azblob -f 
c8y tenantoptions create --category c8y-devmgmt-repo-intgr --key fwUrlExpirationMins --value 5 -f 
c8y tenantoptions create --category c8y-devmgmt-repo-intgr --key fwStorageObserveIntervalMins --value 5 -f 
