credential-provider-echo-token is an image credential provider that uses
Kubernetes service account tokens directly as image registry passwords.

Usage:

    request='{"apiVersion":"credentialprovider.kubelet.k8s.io/v1","kind":"CredentialProviderRequest","image":"...","serviceAccountToken":"..."}'
    echo "${request}" | credential-provider-echo-token [--username=USER]

credential-provider-echo-token is called with STDIN of a JSON-serialized
credentialprovider.kubelet.k8s.io/v1 CredentialProviderRequest,
which must contain a `serviceAccountToken` value. For example:

```
{
  "apiVersion": "credentialprovider.kubelet.k8s.io/v1",
  "kind": "CredentialProviderRequest",
  "image": "...",
  "serviceAccountToken": "..."
}
```

To configure this credential plugin on a node:

1. Create a directory for credential plugin binaries, and place this binary in that directory

2. Create a CredentialProviderConfig configuration file containing:

```
kind: CredentialProviderConfig
apiVersion: kubelet.config.k8s.io/v1
providers:
- name: credential-provider-echo-token
  apiVersion: credentialprovider.kubelet.k8s.io/v1
  tokenAttributes:
    requireServiceAccount: true
    serviceAccountTokenAudience: "" # TODO: replace with the audience your registry expects
    cacheType: Token

  matchImages:
  - "" # TODO: replace with all the registry name(s) / pattern(s) this credential provider should be used with

  # optionally specify the username to include in registry credentials, default is "" if unset
  args:
  - "--username="
```

3. Adjust the kubelet startup flags to point at that configuration file:

```
--image-credential-provider-bin-dir=/path/to/step-1/directory
--image-credential-provider-config=/path/to/step-2/file
```