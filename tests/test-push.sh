#!/bin/bash
# Test pushing to an uninitialized repository
set -eo pipefail

make install
rm -rf /tmp/gitservice && mkdir -p /tmp/gitservice
cd /tmp/gitservice
git init sourcerepo && cd sourcerepo
echo "hello world" > README.md
git add README.md && git commit -a -m"Start repo"
gitservicecli tx gitService push-refs aknudsen/test +head:refs/heads/master --chain-id test --from $(gitservicecli keys show aknudsen --address)
