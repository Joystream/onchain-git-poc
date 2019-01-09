#!/bin/bash
# Test pushing a fast-forward update
set -eo pipefail

make install

rm -rf /tmp/gitservice && mkdir -p /tmp/gitservice
cd /tmp/gitservice
git init -q --bare targetrepo.git
git init -q sourcerepo && cd sourcerepo
git remote add origin debugf:///tmp/gitservice/targetrepo.git

echo "#Hello World" > README.md
git add README.md && git commit -q -m"Start repo"
gogitclient push origin +refs/heads/master:refs/heads/master

echo "This is a test of pushing branch updates" >> README.md
git add README.md && git commit -q -m"Edit README"
gogitclient push origin refs/heads/master:refs/heads/master
