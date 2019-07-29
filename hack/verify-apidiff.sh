#!/usr/bin/env bash
# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
export KUBE_ROOT
source "${KUBE_ROOT}/hack/lib/init.sh"

export GO111MODULE=on

extract_api() {
    module=$1
    full_package=$2
    relative_package=${full_package#"${module}/"}
    apidiff_filename=$(echo ${relative_package} | tr / .)
    mkdir -p apidiff/HEAD
    apidiff -w apidiff/HEAD/${apidiff_filename} ${full_package}
}
export -f extract_api

api_repos=(
    k8s.io/apimachinery
    k8s.io/client-go
)

go install golang.org/x/exp/cmd/apidiff@latest

for module in ${api_repos[*]}; do
    echo "${module} =========="
    pushd "staging/src/${module}" >/dev/null
        # uncomment to regenerate for HEAD
        go list ./pkg/... 2>/dev/null | GO111MODULE=on xargs -n 1 bash -c 'extract_api "$@"' _ "${module}"

        for version in $(ls apidiff | sort | grep -v HEAD); do
          echo "${version} ====="
          pushd "apidiff/${version}" >/dev/null
            for file in $(ls | sort); do
              incompatible=$(apidiff -incompatible "./${file}" "../HEAD/${file}")
              if [[ -n "${incompatible}" ]]; then
                echo "${file}:"
                echo "${incompatible}"
              fi
            done
          popd
        done
    popd >/dev/null
done
