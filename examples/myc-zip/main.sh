#!/bin/sh

set -ve

# build the hello world example and put the zip file in this directory
sp build mypkg.zip .

# inspect the zip file using myczip
myc zip inspect mypkg.zip
