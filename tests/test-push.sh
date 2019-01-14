#!/bin/bash
# Test pushing an initial commit
set -eo pipefail

make install
rm -rf /tmp/gitservice && mkdir -p /tmp/gitservice
cd /tmp/gitservice
git init sourcerepo && cd sourcerepo
echo "#Hello World" > README.md
git add README.md && git commit -m"Start repo"
# Note that we force push to overwrite old content (by prefixing with '+')
gitservicecli tx gitService push-refs aknudsen/test +head:refs/heads/master --chain-id test --from $(gitservicecli keys show aknudsen --address)
